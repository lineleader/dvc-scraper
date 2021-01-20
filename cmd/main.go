package main

import (
	"fmt"
	"log"

	"github.com/davecgh/go-spew/spew"
	"github.com/gobuffalo/envy"
	dvcscraper "github.com/lineleader/dvc-scraper"
)

func main() {
	email := envy.Get("EMAIL", "")
	password := envy.Get("PASSWORD", "")

	scraper, err := dvcscraper.New()
	if err != nil {
		err = fmt.Errorf("failed to start scraper: %w", err)
		log.Fatal(err)
	}
	fmt.Println("Started scraper")

	err = scraper.Login(email, password)
	if err != nil {
		err = fmt.Errorf("failed to sign into DVC: %w", err)
		log.Fatal(err)
	}

	fmt.Println("Signed in!")

	prices, err := scraper.GetPurchasePrices()
	if err != nil {
		err = fmt.Errorf("failed to get purchase prices: %w", err)
		log.Fatal(err)
	}

	spew.Dump(prices)
}
