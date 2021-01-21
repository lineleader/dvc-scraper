package dvcscraper

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/go-rod/bypass"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const (
	signinURL = "https://disneyvacationclub.disney.go.com/sign-in/"
	addOnURL  = "https://disneyvacationclub.disney.go.com/add-vacation-points/"

	dashboardCheckSelector = ".memberNewsAlert"
	signInEmailSelector    = ".field-username-email input"
	signInPasswordSelector = ".field-password input"
	signInSubmitSelector   = ".workflow-login .btn-submit"

	resortCardsSelector = ".resortListItem"
	resortPriceSelector = ".resortPricing"
	resortNameSelector  = ".resortTileDetails h3"
)

type Scraper struct {
	browser *rod.Browser
}

type ResortPrice struct {
	Name          string  `json:"name"`
	PricePerPoint float64 `json:"price_per_point"`
}

type Elementable interface {
	Element(string) (*rod.Element, error)
}

func New() (Scraper, error) {
	var scraper Scraper

	scraper.browser = rod.New()
	// scraper.browser.ServeMonitor(":9777")

	err := scraper.browser.Connect()
	return scraper, err
}

func (s *Scraper) GetPurchasePrices() ([]ResortPrice, error) {
	prices := []ResortPrice{}
	page, err := s.getPage()
	if err != nil {
		err = fmt.Errorf("failed to get bypass page: %w", err)
		return prices, err
	}

	err = page.Navigate(addOnURL)
	if err != nil {
		err = fmt.Errorf("failed to visit add-on tool page: %w", err)
		return prices, err
	}

	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  2560,
		Height: 1400,
	})
	if err != nil {
		err = fmt.Errorf("failed to set viewport: %w", err)
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

	return prices, nil
}

func (s *Scraper) Close() error {
	return s.browser.Close()
}

func (s *Scraper) Login(email, password string) error {
	page, err := s.getPage()
	if err != nil {
		err = fmt.Errorf("failed to get bypass page: %w", err)
		return err
	}

	err = page.Navigate(signinURL)
	if err != nil {
		err = fmt.Errorf("failed to visit sign in page: %w", err)
		return err
	}

	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  2560,
		Height: 1400,
	})
	if err != nil {
		err = fmt.Errorf("failed to set viewport: %w", err)
		return err
	}

	frame, err := getIFrame(page, `iframe[id="disneyid-iframe"]`)
	if err != nil {
		err = fmt.Errorf("failed to get iframe page: %w", err)
		return err
	}

	err = typeInput(frame, signInEmailSelector, email)
	if err != nil {
		err = fmt.Errorf("failed to input email address: %w", err)
		return err
	}

	err = typeInput(frame, signInPasswordSelector, password)
	if err != nil {
		err = fmt.Errorf("failed to input password: %w", err)
		return err
	}

	err = click(frame, signInSubmitSelector)
	if err != nil {
		err = fmt.Errorf("failed to click to sign in: %w", err)
		return err
	}

	_, err = page.Race().Element(dashboardCheckSelector).Do()
	if err != nil {
		err = fmt.Errorf("failed to confirm sign in : %w", err)
		return err
	}

	return nil
}

func click(page *rod.Page, selector string) error {
	clickElem, err := page.Element(selector)
	if err != nil {
		err = fmt.Errorf("failed to get element for click: %w", err)
		return err
	}

	return clickElem.Click(proto.InputMouseButtonLeft)
}

func typeInput(page *rod.Page, selector, text string) error {
	inputElem, err := page.Element(selector)
	if err != nil {
		err = fmt.Errorf("failed to get element for input: %w", err)
		return err
	}

	return inputElem.Input(text)
}

func getIFrame(page *rod.Page, selector string) (*rod.Page, error) {
	frameElem, err := page.Element(selector)
	if err != nil {
		err = fmt.Errorf("failed to get iframe element: %w", err)
		return nil, err
	}

	frame, err := frameElem.Frame()
	if err != nil {
		err = fmt.Errorf("failed to get iframe page: %w", err)
		return nil, err
	}

	return frame, nil
}

func (s *Scraper) getPage() (*rod.Page, error) {
	return bypass.Page(s.browser)
}

func textOfElement(page Elementable, selector string) (string, error) {
	elem, err := page.Element(selector)
	if err != nil {
		err = fmt.Errorf("failed to get element for text: %w", err)
		return "", err
	}

	return elem.Text()
}
