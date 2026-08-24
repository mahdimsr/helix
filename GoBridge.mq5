#property strict

string  Host       = "127.0.0.1";
int     Port       = 8585;
string  Symbol_    = SYMBOL_CURRENCY_BASE;
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

void HandleCommands(int socket)
{
   uint len = SocketIsReadable(socket);
   if(len == 0) return;

   uchar buf[];
   int read = SocketRead(socket, buf, len, 500);
   if(read <= 0) return;

   string incoming = CharArrayToString(buf, 0, read, CP_UTF8);
   
   // حذف فاصله‌ها و اینترهای اضافی از ابتدا و انتهای کل رشته
   StringTrimRight(incoming);
   StringTrimLeft(incoming);
   
   if(StringLen(incoming) == 0) return;

   // ۱. ابتدا رشته را بر اساس خط جدید (\n) به دستورات جداگانه تقسیم می‌کنیم
   string lines[];
   int totalLines = StringSplit(incoming, '\n', lines);

   // ۲. هر خط (دستور) را جداگانه پردازش می‌کنیم
   for(int i = 0; i < totalLines; i++)
   {
      string cmdLine = lines[i];
      StringTrimRight(cmdLine);
      StringTrimLeft(cmdLine);
      
      if(StringLen(cmdLine) == 0) continue; // خط خالی را رد کن

      Print("📩 Processing command: ", cmdLine);

      string parts[];
      int n = StringSplit(cmdLine, '|', parts);
      if(n <= 0) continue;

      string command = parts[0];
      
      if(command == "GET_CANDLES" && n >= 4)
      {   
         string symbol = parts[1];
         string timeframeString = parts[2];
         int count = (int)StringToInteger(parts[3]);

         ENUM_TIMEFRAMES timeframe = StringToTimeframe(timeframeString);
         if(timeframe == PERIOD_CURRENT)
         {
            Print("Invalid timeframe: ", timeframeString);
            continue;
         }

         string json = GetCandlesJSON(symbol, timeframe, count);
         if(json != "")
         {
            if(SendCandles(json))
                Print("✅ Sent ", StringLen(json), " bytes JSON for candles");
            else
                Print("❌ Failed to send candles");
         }
      }
      else if(command == "PLACE_ORDER")
      {
         if(n < 6) { Print("Invalid PLACE_ORDER format"); continue; }
         
         string symbol = parts[1];
         string side = parts[2];
         double lot = StringToDouble(parts[3]);
         double tp = StringToDouble(parts[4]);
         double sl = StringToDouble(parts[5]);
         
         SendOrderResult(symbol, side, lot, tp, sl);
      }
      else if(command == "UPDATE_ORDER")
      {
         if(n < 4)
         {
            SendStr("{\"type\":\"UPDATE_ORDER\",\"data\":{\"success\":false,\"comment\":\"Invalid format\"}}\n");
            continue;
         }
         
         ulong ticket = StringToInteger(parts[1]);
         double new_sl = StringToDouble(parts[2]);
         double new_tp = StringToDouble(parts[3]);
         
         if(!PositionSelectByTicket(ticket))
         {
            SendStr("{\"type\":\"UPDATE_ORDER\",\"data\":{\"success\":false,\"ticket\":" + IntegerToString(ticket) + ",\"comment\":\"Position not found\"}}\n");
            continue;
         }
         
         string symbol = PositionGetString(POSITION_SYMBOL);
         UpdateOrderTpSl(symbol, ticket, new_tp, new_sl);
      }
      else if(command == "INQUIRY" && n >= 2)
      {
         ulong ticket = StringToInteger(parts[1]);
         HandleInquiry(ticket);
      }
      else
      {
         Print("⚠️ Unknown command: ", command);
      }
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

void SendOrderResult(string symbol, string side, double lot, double tp, double sl)
{
    MqlTradeRequest req;
    MqlTradeResult  res;
    ZeroMemory(req);
    ZeroMemory(res);
    
    double ask = SymbolInfoDouble(symbol, SYMBOL_ASK);
    double bid = SymbolInfoDouble(symbol, SYMBOL_BID);
    
    
    req.action       = TRADE_ACTION_DEAL;
    req.symbol       = symbol;
    req.volume       = lot;
    req.type         = (side == "BUY") ? ORDER_TYPE_BUY : ORDER_TYPE_SELL;
    req.price        = (side == "BUY") ? ask : bid;
    req.deviation    = 10;
    // req.magic        = 123456;
    req.type_filling = GetFillingMode(symbol);
    
    if(tp > 0) req.tp = tp;
    if(sl > 0) req.sl = sl;

    bool ok = OrderSend(req, res);
    
    ulong position_ticket = 0;
    
    if(ok && res.deal > 0)
    {
        if(HistorySelect(TimeCurrent() - 60, TimeCurrent() + 60))
        {
            position_ticket = HistoryDealGetInteger(res.deal, DEAL_POSITION_ID);
        }
    }
    
    if(position_ticket == 0)
    {
      position_ticket = res.order;
    }
    
    Print("order send result is: " + ok + "\n");
    Print("OrderSend => ok=", ok,
      " retcode=", res.retcode,
      " comment=", res.comment,
      " ask=", ask, " bid=", bid,
      " tp=", tp, " sl=", sl);
    
    string envelope = StringFormat(
      "{\"type\":\"ORDER\",\"data\":{\"success\":%s,\"ticket\":%I64d,\"retcode\":%d,\"price\":%.5f,\"tp\":%.5f,\"sl\":%.5f,\"comment\":\"%s\"}} \n",
      ok ? "true" : "false",
      position_ticket,
      res.retcode,
      res.price,
      req.tp,
      req.sl,
      res.comment
    );

    SendLargeString(envelope);
}

void UpdateOrderTpSl(string symbol, ulong ticket, double tp, double sl)
{
    MqlTradeRequest req = {};
    MqlTradeResult res = {};
    
    req.action = TRADE_ACTION_SLTP;
    req.position = ticket;
    req.symbol = symbol;
    req.sl = sl;
    req.tp = tp;
    
    bool ok = OrderSend(req, res);
    
    Print("update order result is: " + ok + "\n");
    
    string envelope = StringFormat(
      "{\"type\":\"UPDATE_ORDER\",\"data\":{\"success\":%s,\"ticket\":%I64d,\"retcode\":%d,\"price\":%.5f,\"tp\":%.5f,\"sl\":%.5f,\"comment\":\"%s\"}} \n",
      ok ? "true" : "false",
      res.deal > 0 ? res.deal : res.order,
      res.retcode,
      res.price,
      req.tp,
      req.sl,
      res.comment
    );

    SendLargeString(envelope);
}


void HandleInquiry(ulong ticket)
{
    bool found = false;
    int status_code = 3000; // 3000 = NOT_FOUND
    double current_price = 0;
    double tp = 0;
    double sl = 0;
    string comment_info = "Not Found";
    
    double balance = AccountInfoDouble(ACCOUNT_BALANCE);
    double equity = AccountInfoDouble(ACCOUNT_EQUITY);
    
    Print("🔍 Inquiry started for ticket: ", ticket);
    
    // ۱. بررسی پوزیشن‌های باز
    if(PositionSelectByTicket(ticket))
    {
        found = true;
        status_code = 1000; // 1000 = OPEN
        current_price = PositionGetDouble(POSITION_PRICE_CURRENT);
        tp = PositionGetDouble(POSITION_TP);
        sl = PositionGetDouble(POSITION_SL);
        
        string symbol = PositionGetString(POSITION_SYMBOL);
        string type = (PositionGetInteger(POSITION_TYPE) == POSITION_TYPE_BUY) ? "BUY" : "SELL";
        double volume = PositionGetDouble(POSITION_VOLUME);
        double profit = PositionGetDouble(POSITION_PROFIT);
        
        comment_info = StringFormat("STATUS:OPEN | SYM:%s | TYPE:%s | VOL:%.2f | PRF:%.2f", 
                                    symbol, type, volume, profit);
                                    
        Print("✅ Found as OPEN Position");
    }
    else
    {
        // ۲. بررسی تاریخچه (۹۰ روز اخیر)
        HistorySelect(TimeCurrent() - 90*24*60*60, TimeCurrent());
        
        bool foundInHistory = false;
        
        // الف) جستجو در Deals بر اساس POSITION_ID (برای پوزیشن‌های بسته شده)
        int totalDeals = HistoryDealsTotal();
        for(int i = totalDeals - 1; i >= 0; i--)
        {
            ulong dealTicket = HistoryDealGetTicket(i);
            if(dealTicket == 0) continue;
            
            // چک می‌کنیم آیا این Deal متعلق به Position مورد نظر ما است
            ulong positionId = (ulong)HistoryDealGetInteger(dealTicket, DEAL_POSITION_ID);
            
            if(positionId == ticket)
            {
                found = true;
                foundInHistory = true;
                status_code = 2000; // CLOSED
                
                current_price = HistoryDealGetDouble(dealTicket, DEAL_PRICE);
                tp = 0; 
                sl = 0;
                
                string symbol = HistoryDealGetString(dealTicket, DEAL_SYMBOL);
                ENUM_DEAL_TYPE dealType = (ENUM_DEAL_TYPE)HistoryDealGetInteger(dealTicket, DEAL_TYPE);
                ENUM_DEAL_ENTRY dealEntry = (ENUM_DEAL_ENTRY)HistoryDealGetInteger(dealTicket, DEAL_ENTRY);
                
                string type = "UNKNOWN";
                if(dealType == DEAL_TYPE_BUY) type = "BUY";
                else if(dealType == DEAL_TYPE_SELL) type = "SELL";
                
                double volume = HistoryDealGetDouble(dealTicket, DEAL_VOLUME);
                double profit = HistoryDealGetDouble(dealTicket, DEAL_PROFIT);
                double swap = HistoryDealGetDouble(dealTicket, DEAL_SWAP);
                double commission = HistoryDealGetDouble(dealTicket, DEAL_COMMISSION);
                string deal_comment = HistoryDealGetString(dealTicket, DEAL_COMMENT);
                
                // تشخیص جهت معامله (باز شدن یا بسته شدن)
                string entryInfo = "";
                if(dealEntry == DEAL_ENTRY_IN) entryInfo = "[OPEN]";
                else if(dealEntry == DEAL_ENTRY_OUT) entryInfo = "[CLOSE]";
                else if(dealEntry == DEAL_ENTRY_INOUT) entryInfo = "[REVERSE]";
                else if(dealEntry == DEAL_ENTRY_OUT_BY) entryInfo = "[CLOSE_BY]";

                comment_info = StringFormat(
                    "STATUS:CLOSED %s | SYM:%s | TYPE:%s | VOL:%.2f | PRF:%.2f | SWP:%.2f | COM:%.2f | REASON:%s", 
                    entryInfo, symbol, type, volume, profit, swap, commission, CleanJsonString(deal_comment)
                );
                
                Print("✅ Found in History Deal #", dealTicket, " | Position ID: ", positionId, " | Comment: ", deal_comment);
                break; // اولین Deal مربوط به این پوزیشن را پیدا کردیم، حلقه را بشکن
            }
        }
        
        // ب) اگر در Deals پیدا نشد، شاید یک Pending Order بوده که کنسل یا اجرا شده
        if(!foundInHistory)
        {
            bool orderExists = HistoryOrderSelect(ticket);
            if(orderExists)
            {
                found = true;
                status_code = 2000;
                
                current_price = HistoryOrderGetDouble(ticket, ORDER_PRICE_CURRENT);
                tp = HistoryOrderGetDouble(ticket, ORDER_TP);
                sl = HistoryOrderGetDouble(ticket, ORDER_SL);
                
                string symbol = HistoryOrderGetString(ticket, ORDER_SYMBOL);
                double volume = HistoryOrderGetDouble(ticket, ORDER_VOLUME_INITIAL);
                string order_comment = HistoryOrderGetString(ticket, ORDER_COMMENT);
                
                ENUM_ORDER_STATE state = (ENUM_ORDER_STATE)HistoryOrderGetInteger(ticket, ORDER_STATE);
                string statusStr = EnumToString(state);
                
                comment_info = StringFormat(
                    "STATUS:%s | SYM:%s | VOL:%.2f | REASON:%s", 
                    statusStr, symbol, volume, CleanJsonString(order_comment)
                );
                
                Print("✅ Found in History Order #", ticket, " | State: ", statusStr);
            }
        }
        
        Print("🔍 Search complete. Found in history: ", foundInHistory, " | Final found status: ", found);
    }

    // ساخت JSON نهایی
    string json = StringFormat(
        "{\"type\":\"INQUIRY\",\"data\":{" +
        "\"success\":%s," +
        "\"retcode\":%d," +
        "\"price\":%.5f," +
        "\"tp\":%.5f," +
        "\"sl\":%.5f," +
        "\"ticket\":%I64u," +
        "\"balance\":%0.2f," +
        "\"equity\":%0.2f," +
        "\"comment\":\"%s\"" +
        "}}\n",
        found ? "true" : "false",
        status_code,
        current_price,
        tp,
        sl,
        ticket,
        balance,
        equity,
        comment_info
    );

    SendLargeString(json);
}


// تابع کمکی برای جلوگیری از شکستن JSON
string CleanJsonString(string str)
{
    StringReplace(str, "\\", "\\\\");
    StringReplace(str, "\"", "\\\"");
    StringReplace(str, "\n", "\\n");
    StringReplace(str, "\r", "\\r");
    StringReplace(str, "\t", "\\t");
    return str;
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

ENUM_ORDER_TYPE_FILLING GetFillingMode(string sym)
{
    long filling = SymbolInfoInteger(sym, SYMBOL_FILLING_MODE);
    if((filling & SYMBOL_FILLING_FOK) == SYMBOL_FILLING_FOK)
        return ORDER_FILLING_FOK;
    if((filling & SYMBOL_FILLING_IOC) == SYMBOL_FILLING_IOC)
        return ORDER_FILLING_IOC;
    return ORDER_FILLING_RETURN;
}