package walking

// LiveConfig پیکربندی بهینه‌سازی برای حالت لایو
type LiveConfig struct {
	LookbackDays    int       // تعداد روزهای گذشته برای بهینه‌سازی
	TimeframeMinute int       // دقیقه هر کندل (مثلاً 15 برای M15)
	TPRange         []float64 // محدوده TP دلاری
	SLRange         []float64 // محدوده SL دلاری
	InitialCapital  float64   // سرمایه اولیه برای محاسبه Score
	Leverage        float64   // لوریج برای محاسبه Score
}

// DefaultLiveConfig پیکربندی پیش‌فرض (قابل تنظیم بر اساس بک‌تست)
func DefaultLiveConfig() LiveConfig {
	return LiveConfig{
		LookbackDays:    30, // 30 روز گذشته
		TimeframeMinute: 15, // تایم‌فریم M15
		TPRange:         makeRange(10.0, 100.0, 1),
		SLRange:         makeRange(10.0, 50.0, 1),
		InitialCapital:  1000.0,
		Leverage:        10.0,
	}
}

// CalculateLookbackCount تعداد کندل‌های Lookback را محاسبه می‌کند
func (c *LiveConfig) CalculateLookbackCount() int {
	// تعداد کندل در روز = (24 ساعت * 60 دقیقه) / دقیقه هر کندل
	candlesPerDay := (24 * 60) / c.TimeframeMinute
	return c.LookbackDays * candlesPerDay
}
