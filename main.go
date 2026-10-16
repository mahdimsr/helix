package main

import (
	"helix/exchange"
	"log"
)

func main() {

	/*db := database.MongoConnect()
	candlesRepo := database.NewCandleRepository(&db)

	candles, err := exchange.GetAllCandlesByDateString("XAUTUSDT", "15m", "2026-01-01", "2026-07-01")
	if err != nil {
		log.Fatal(err)
	}

	err = candlesRepo.CreateMany(context.Background(), candles)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Done")*/

	err := exchange.ImportMT4CSVToMongoDB("XAUUSD_M15.csv", "XAUUSD", "15m")

	if err != nil {
		log.Fatal(err)
	}

	log.Print("Done")

}
