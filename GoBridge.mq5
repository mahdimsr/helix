#property strict

input string  Host       = "127.0.0.1";
input int     Port       = 8585;
input string  Symbol_    = "BTCUSD.ecn";   // اسم دقیق نماد رو از Market Watch بگیر
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
   else Print("connected to Go");
}

void OnTimer()
{
   if(sock == INVALID_HANDLE) { Connect(); return; }

   datetime t[];
   if(CopyTime(Symbol_, TF, 0, 1, t) != 1) return;

   // فقط وقتی کندل جدید باز شد یعنی کندل قبلی بسته شده
   if(t[0] == lastBar) return;
   lastBar = t[0];

   MqlRates r[];
   if(CopyRates(Symbol_, TF, 1, 1, r) != 1) return; // کندل بسته‌شده (index 1)

   string msg = StringFormat(
      "{\"symbol\":\"%s\",\"time\":%I64d,\"open\":%.2f,\"high\":%.2f,\"low\":%.2f,\"close\":%.2f,\"volume\":%I64d}\n",
      Symbol_, (long)r[0].time, r[0].open, r[0].high, r[0].low, r[0].close, (long)r[0].tick_volume);

   if(!SendStr(msg)) { Reconnect(); return; }

   HandleEcho(sock);
}

bool SendStr(string s)
{
   uchar buf[];
   int len = StringToCharArray(s, buf, 0, WHOLE_ARRAY, CP_UTF8) - 1;
   return SocketSend(sock, buf, len) == len;
}

string RecvLine()
{
   string line = "";
   uint timeout = GetTickCount() + 2000;
   while(GetTickCount() < timeout)
   {
      uint avail = SocketIsReadable(sock);
      if(avail > 0)
      {
         uchar buf[];
         int n = SocketRead(sock, buf, avail, 100);
         for(int i = 0; i < n; i++)
         {
            if(buf[i] == '\n') return line;
            line += CharToString(buf[i]);
         }
      }
      Sleep(10);
   }
   return line;
}



// پارسر خیلی ساده برای فیلد رشته‌ای
string JsonStr(string json, string key)
{
   string pat = "\"" + key + "\":\"";
   int p = StringFind(json, pat);
   if(p < 0) return "";
   p += StringLen(pat);
   int e = StringFind(json, "\"", p);
   if(e < 0) return "";
   return StringSubstr(json, p, e - p);
}

void Reconnect()
{
   if(sock != INVALID_HANDLE) SocketClose(sock);
   sock = INVALID_HANDLE;
}

// دریافت دستور از Go، اجرا، و ارسال نتیجه
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
   req.volume       = 0.1;
   req.type         = (cmd == "BUY") ? ORDER_TYPE_BUY : ORDER_TYPE_SELL;
   req.price        = price;
   req.deviation    = 500;
   req.magic        = 12345;
   req.type_filling = ORDER_FILLING_IOC;
   req.type_time    = ORDER_TIME_GTC;
   req.tp = price + 500;
   req.sl = price - 2500;

   long minStop = SymbolInfoInteger(_Symbol, SYMBOL_TRADE_STOPS_LEVEL);
   double minDist = minStop * _Point;
   PrintFormat("min stop dist = %.2f", minDist);

   bool ok = OrderSend(req, res);

   PrintFormat("OrderSend ok=%d retcode=%d ticket=%I64u comment=%s tp:%0.1f sl:%0.1f",
               ok, res.retcode, res.deal, res.comment, req.tp, req.sl);
}

// ساخت JSON نتیجه و نوشتن روی سوکت
void SendResult(int socket, bool ok, uint retcode, ulong ticket, string comment)
{
   string json = StringFormat(
      "{\"ok\":%s,\"retcode\":%d,\"ticket\":%I64u,\"comment\":\"%s\"}\n",
      (ok ? "true" : "false"), retcode, ticket, comment);

   if(!SendStr(json)) { Reconnect(); return; }
}

// خواندن یک پیام کامل از سوکت (تا newline یا تمام شدن بافر)
string ReadFromSocket(int socket)
{
   string result = "";
   uchar buf[];
   uint  len;

   while((len = SocketIsReadable(socket)) > 0)
   {
      int read = SocketRead(socket, buf, len, 1000);
      if(read > 0)
         result += CharArrayToString(buf, 0, read, CP_UTF8);
      else
         break;
   }
   return result;
}
   
// ارسال یک رشته به سوکت
bool WriteToSocket(int socket, string msg)
{
   uchar buf[];
   int len = StringToCharArray(msg, buf, 0, WHOLE_ARRAY, CP_UTF8);
   // حذف null terminator انتهای که StringToCharArray اضافه می‌کند
   if(len > 0 && buf[len-1] == 0)
      ArrayResize(buf, len-1);
   return SocketSend(socket, buf, ArraySize(buf)) > 0;
}

// حلقه اصلی echo: هرچه از Go بیاید + " from metatrader" را برمی‌گرداند
void HandleEcho(int socket)
{
   string incoming = ReadFromSocket(socket);
   if(StringLen(incoming) == 0)
      return;
   
   StringTrimLeft(incoming);
   StringTrimRight(incoming);
   
   string side = "";
   if(StringFind(incoming,"BUY") >= 0){
      side = "BUY";
   }else if(StringFind(incoming,"SELL")){
      side = "SELL";
   }

   PrintFormat("Received from Go: %s", side);
   
   HandlePlaceOrder(socket,side,0.03,0.03);

   string reply = side + " from metatrader";
   if(WriteToSocket(socket, reply))
      PrintFormat("Sent back: %s", reply);
   else
      Print("WriteToSocket failed, err=", GetLastError());
}