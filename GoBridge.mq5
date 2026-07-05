#property strict

input string  Host       = "127.0.0.1";
input int     Port       = 8585;
input string  Symbol_    = SYMBOL_CURRENCY_BASE;
input double  Lot        = 0.1;
input ENUM_TIMEFRAMES TF = PERIOD_M1;

int      sock = INVALID_HANDLE;
datetime lastBar = 0;

int OnInit()
{
   Connect();
   EventSetTimer(1);
   return INIT_SUCCEEDED;
}

void OnDeinit(const int reason)
{
   EventKillTimer();
   if(sock != INVALID_HANDLE) SocketClose(sock);
}

void Connect()
{
   sock = SocketCreate();
   if(sock == INVALID_HANDLE) { Print("SocketCreate failed"); return; }
   if(!SocketConnect(sock, Host, Port, 1000))
   {
      Print("SocketConnect failed");
      SocketClose(sock);
      sock = INVALID_HANDLE;
   }
   else Print("Socket Connected...");
}

void OnTimer()
{
   if(sock == INVALID_HANDLE) { Connect(); return; }

   // چک کردن دستورات از Go
   HandleCommands(sock);

   datetime t[];
   if(CopyTime(Symbol_, TF, 0, 1, t) != 1) return;

   if(t[0] == lastBar) return;
   lastBar = t[0];

   MqlRates r[];
   if(CopyRates(Symbol_, TF, 1, 1, r) != 1) return;

   string msg = StringFormat(
      "{\"symbol\":\"%s\",\"time\":%I64d,\"open\":%.2f,\"high\":%.2f,\"low\":%.2f,\"close\":%.2f,\"volume\":%I64d}\n",
      SYMBOL_CURRENCY_BASE, (long)r[0].time, r[0].open, r[0].high, r[0].low, r[0].close, (long)r[0].tick_volume);

   if(!SendStr(msg)) { Reconnect(); return; }
}

bool SendStr(string s)
{
   uchar buf[];
   int len = StringToCharArray(s, buf, 0, WHOLE_ARRAY, CP_UTF8) - 1;
   return SocketSend(sock, buf, len) == len;
}

void Reconnect()
{
   if(sock != INVALID_HANDLE) SocketClose(sock);
   sock = INVALID_HANDLE;
}

// خواندن دستورات از Go و اجرای آن‌ها
void HandleCommands(int socket)
{
   uint len = SocketIsReadable(socket);
   if(len == 0) return;

   string incoming = "";
   uchar buf[];
   int read = SocketRead(socket, buf, len, 500);
   if(read > 0)
      incoming = CharArrayToString(buf, 0, read, CP_UTF8);

   if(StringLen(incoming) == 0) return;

   StringTrimLeft(incoming);
   StringTrimRight(incoming);

   Print("Received from Go: ", incoming);

   // پارس دستور: GET_CANDLES|symbol|timeframe|count
   string parts[];
   int n = StringSplit(incoming, '|', parts);

   if(n >= 4 && parts[0] == "GET_CANDLES")
   {
      string symbol = parts[1];
      string tfStr = parts[2];
      int count = (int)StringToInteger(parts[3]);

      ENUM_TIMEFRAMES tf = StringToTimeframe(tfStr);
      if(tf == PERIOD_CURRENT)
      {
         Print("Invalid timeframe: ", tfStr);
         return;
      }

      string json = GetCandlesJSON(symbol, tf, count);
      if(json != "")
      {
         if(SendCandles(json))
             Print("Sent ", StringLen(json), " bytes JSON");
         else
            Print("Failed to send candles");
      }
      return;
   }

   // دستور BUY یا SELL
   if(StringFind(incoming, "BUY") >= 0)
   {
      HandlePlaceOrder(socket, "BUY", 0.03, 0.03);
   }
   else if(StringFind(incoming, "SELL") >= 0)
   {
      HandlePlaceOrder(socket, "SELL", 0.03, 0.03);
   }
}

ENUM_TIMEFRAMES StringToTimeframe(string tfStr)
{
   if(tfStr == "PERIOD_M1")  return PERIOD_M1;
   if(tfStr == "PERIOD_M5")  return PERIOD_M5;
   if(tfStr == "PERIOD_M15") return PERIOD_M15;
   if(tfStr == "PERIOD_M30") return PERIOD_M30;
   if(tfStr == "PERIOD_H1")  return PERIOD_H1;
   if(tfStr == "PERIOD_H4")  return PERIOD_H4;
   if(tfStr == "PERIOD_D1")  return PERIOD_D1;
   if(tfStr == "PERIOD_W1")  return PERIOD_W1;
   if(tfStr == "PERIOD_MN1") return PERIOD_MN1;
   return PERIOD_CURRENT;
}

string GetCandlesJSON(string symbol, ENUM_TIMEFRAMES tf, int count)
{
   MqlRates rates[];
   ArraySetAsSeries(rates, true);

   int copied = CopyRates(symbol, tf, 0, count, rates);
   if(copied <= 0)
   {
      Print("CopyRates failed. Error: ", GetLastError());
      return "";
   }

   string json = "[";
   for(int i = 0; i < copied; i++)
   {
      if(i > 0) json += ",";

      json += StringFormat("{\"time\":%I64d,\"open\":%.5f,\"high\":%.5f,\"low\":%.5f,\"close\":%.5f,\"volume\":%I64d}",
                           (long)rates[i].time,
                           rates[i].open,
                           rates[i].high,
                           rates[i].low,
                           rates[i].close,
                           rates[i].tick_volume);
   }
   json += "]";

   return json;
}

void HandlePlaceOrder(int socket, string cmd, float tpPercent, float slPercent)
{
   MqlTradeRequest req;
   MqlTradeResult  res;
   ZeroMemory(req);
   ZeroMemory(res);

   double price = (cmd == "BUY")
                  ? SymbolInfoDouble(_Symbol, SYMBOL_ASK)
                  : SymbolInfoDouble(_Symbol, SYMBOL_BID);

   req.action       = TRADE_ACTION_DEAL;
   req.symbol       = _Symbol;
   req.volume       = Lot;
   req.type         = (cmd == "BUY") ? ORDER_TYPE_BUY : ORDER_TYPE_SELL;
   req.price        = price;
   req.deviation    = 500;
   req.magic        = 12345;
   req.type_filling = ORDER_FILLING_IOC;
   req.type_time    = ORDER_TIME_GTC;

   // محاسبه TP/SL بر اساس درصد
   if(cmd == "BUY")
   {
      req.tp = price + (price * tpPercent);
      req.sl = price - (price * slPercent);
   }
   else
   {
      req.tp = price - (price * tpPercent);
      req.sl = price + (price * slPercent);
   }

   long minStop = SymbolInfoInteger(_Symbol, SYMBOL_TRADE_STOPS_LEVEL);
   double minDist = minStop * _Point;
   PrintFormat("min stop dist = %.2f", minDist);

   bool ok = OrderSend(req, res);

   PrintFormat("OrderSend ok=%d retcode=%d ticket=%I64u comment=%s tp:%.5f sl:%.5f",
               ok, res.retcode, res.deal, res.comment, req.tp, req.sl);

   SendResult(socket, ok, res.retcode, res.deal, res.comment);
}

void SendResult(int socket, bool ok, uint retcode, ulong ticket, string comment)
{
   string json = StringFormat(
      "{\"ok\":%s,\"retcode\":%d,\"ticket\":%I64u,\"comment\":\"%s\"}\n",
      (ok ? "true" : "false"), retcode, ticket, comment);

   if(!SendStr(json))
      Reconnect();
}

bool SendCandles(string candlesJsonString)
{
    string envelope = StringFormat(
        "{\"type\":\"CANDLES\",\"data\":%s}", 
        candlesJsonString
    );
   
   return SendLargeString(envelope + "\n");
}

bool SendLargeString(string s)
{
   uchar buf[];
   int len = StringToCharArray(s, buf, 0, WHOLE_ARRAY, CP_UTF8) - 1;

   int sent = 0;
   int chunkSize = 4096;

   while(sent < len)
   {
      int toSend = (int)MathMin(chunkSize, len - sent);
      uchar chunk[];
      ArrayCopy(chunk, buf, 0, sent, toSend);

      int result = SocketSend(sock, chunk, toSend);
      if(result < 0) return false;

      sent += result;
   }
   return true;
}