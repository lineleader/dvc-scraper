package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gobuffalo/envy"
	dvcscraper "github.com/lineleader/dvc-scraper"
)

var currentPrices = map[string]float64{
	"Aulani, Disney Vacation Club Villas,\nKo Olina, Hawai‘i":   201,
	"Disney's Riviera Resort":                                   201,
	"Copper Creek Villas & Cabins at Disney's Wilderness Lodge": 225,
	"Bay Lake Tower at Disney's Contemporary Resort":            245,
	"Boulder Ridge Villas at Disney's Wilderness Lodge":         186,
	"Disney's Animal Kingdom Villas – Jambo House":              186,
	"Disney's Animal Kingdom Villas – Kidani Village":           186,
	"Disney's Beach Club Villas":                                245,
	"Disney's BoardWalk Villas":                                 210,
	"Disney's Hilton Head Island Resort":                        140,
	"Disney's Old Key West Resort":                              165,
	"Disney's Polynesian Villas & Bungalows":                    250,
	"Disney's Saratoga Springs Resort":                          165,
	"Disney's Vero Beach Resort":                                125,
	"The Villas at Disney's Grand Floridian Resort":             255,
}

func main() {
	email := envy.Get("EMAIL", "")
	password := envy.Get("PASSWORD", "")
	// email := faker.Email()
	// password := faker.Password()

	scraper, err := dvcscraper.New(email, password)
	if err != nil {
		err = fmt.Errorf("failed to start scraper: %w", err)
		log.Fatal(err)
	}
	defer scraper.Close()
	fmt.Println("Started scraper")

	// prices(&scraper)
	// avail(&scraper)

	err = scraper.AuthenticatedNavigate("https://disneyvacationclub.disney.go.com/home/")
	if err != nil {
		err = fmt.Errorf("failed to visit dashboard: %w", err)
		log.Fatal(err)
	}

	scraper.Screenshot("dashboard.png")
	fmt.Println("Done.")
}

func prices(scraper *dvcscraper.Scraper) {
	prices, err := scraper.GetPurchasePrices()
	if err != nil {
		err = fmt.Errorf("failed to get purchase prices: %w", err)
		log.Fatal(err)
	}

	for _, price := range prices {
		currentPrice, ok := currentPrices[price.Name]
		if !ok {
			fmt.Println("\nCurrent price not found!!", price.Name, price.PricePerPoint)
			continue
		}

		if currentPrice != price.PricePerPoint {
			fmt.Printf("Price difference found!!\n%s\n$%.2f => $%.2f\n\n", price.Name, currentPrice, price.PricePerPoint)
		}
	}
}

func avail(scraper *dvcscraper.Scraper) {
	handle, err := scraper.NewAvailabilityHandle()
	if err != nil {
		err = fmt.Errorf("failed to get availability handle: %w", err)
		log.Fatal(err)
	}

	results, err := handle.GetAvailability(dvcscraper.AvailabilityOptions{
		Resort:   "BLT",
		RoomType: "4O",
		Date:     time.Now(),
	})

	fmt.Println("Err:", err)
	fmt.Println("Res:", results)

	fmt.Println("Done.")
}
