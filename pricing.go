package dvcscraper

import (
	"fmt"
	"regexp"
	"strconv"
)

const (
	addOnURL = "https://disneyvacationclub.disney.go.com/add-vacation-points/"

	resortCardsSelector = ".resort-tile"
	resortPriceSelector = ".resort-pricing"
	resortNameSelector  = ".resort-details h3"
)

// ResortPrice models a resort and a dollar per point price
type ResortPrice struct {
	Name          string  `json:"name"`
	PricePerPoint float64 `json:"price_per_point"`
}

// GetPurchasePrices returns current pricing for new contracts with DVC
func (s *Scraper) GetPurchasePrices() ([]ResortPrice, error) {
	prices := []ResortPrice{}

	err := s.AuthenticatedNavigate(addOnURL, resortCardsSelector)
	if err != nil {
		err = fmt.Errorf("failed to visit add-on tool page: %w", err)
		return prices, err
	}

	page, err := s.getPage()
	if err != nil {
		err = fmt.Errorf("failed to get bypass page: %w", err)
		return prices, err
	}

	_, err = page.Race().Element(resortCardsSelector).Do()
	if err != nil {
		err = fmt.Errorf("failed to wait for resort cards: %w", err)
		return prices, err
	}

	resortCards, err := page.Elements(resortCardsSelector)
	if err != nil {
		err = fmt.Errorf("failed to get resort cards: %w", err)
		return prices, err
	}

	var errs []error
	priceRegExp, err := regexp.Compile(`\d+`)
	if err != nil {
		err = fmt.Errorf("failed to compile price regexp: %w", err)
		return prices, err
	}
	for _, card := range resortCards {
		name, err := textOfElement(card, resortNameSelector)
		if err != nil {
			err = fmt.Errorf("failed to get name of resort: %w", err)
			errs = append(errs, err)
		}
		price, err := textOfElement(card, resortPriceSelector)
		if err != nil {
			err = fmt.Errorf("failed to get price of resort: %w", err)
			errs = append(errs, err)
		}
		trimmedPrice := priceRegExp.FindString(price)
		parsedPrice, err := strconv.ParseFloat(trimmedPrice, 64)
		if err != nil {
			err = fmt.Errorf("failed to parse price (%s => %s): %w", price, trimmedPrice, err)
			errs = append(errs, err)
		}
		prices = append(prices, ResortPrice{Name: name, PricePerPoint: parsedPrice})
	}

	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Println("Error: ", err)
		}
		return prices, errs[0]
	}

	return prices, nil
}
