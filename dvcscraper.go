// Package dvcscraper encapsulates common scraping methods for the DVC website
package dvcscraper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
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

	cooieSessionFile = ".dvcscraper-session.json"
)

// Scraper provides authenticated access to the DVC website to scrape data easily
type Scraper struct {
	email    string
	password string

	browser *rod.Browser
}

// ResortPrice models a resort and a dollar per point price
type ResortPrice struct {
	Name          string  `json:"name"`
	PricePerPoint float64 `json:"price_per_point"`
}

type elementable interface {
	Element(string) (*rod.Element, error)
}

// New returns a Scraper ready to roll
func New(email, password string) (Scraper, error) {
	scraper := Scraper{
		email:    email,
		password: password,
	}

	scraper.browser = rod.New()
	// scraper.browser.ServeMonitor(":9777")

	err := scraper.browser.Connect()

	cookieReader, err := os.Open(cooieSessionFile)
	if err != nil {
		err = fmt.Errorf("failed to read cookie session file: %w", err)
		return scraper, err
	}
	defer cookieReader.Close()

	err = scraper.SetCookies(cookieReader)
	if err != nil {
		err = fmt.Errorf("failed to set cookies: %w", err)
		return scraper, err
	}

	return scraper, err
}

// NewWithBinary returns a new Scraper with the provided browser binary launched
func NewWithBinary(email, password, binpath string) (Scraper, error) {
	scraper := Scraper{
		email:    email,
		password: password,
	}

	u, err := launcher.New().Bin(binpath).Launch()
	if err != nil {
		err = fmt.Errorf("failed to : %w", err)
		return scraper, err
	}

	scraper.browser = rod.New()
	// scraper.browser.ServeMonitor(":9777")
	scraper.browser.ControlURL(u)

	err = scraper.browser.Connect()
	return scraper, err
}

// GetCookies returns a JSON encoded set of the current Scraper's browser's cookies.
//
// This is used in conjunction with `SetCookies` to pre- or re-load browser
// state across scraper runs. For example, to bypass logging in each run and reuse
// sessions.
//
// Callers of `GetCookies` should persist the returned bytes and then call
// `SetCookies` later with the same content. This will "resume" where you left off.
func (s *Scraper) GetCookies() ([]byte, error) {
	cookies, err := s.browser.GetCookies()
	if err != nil {
		err = fmt.Errorf("failed to get cookies from browser: %w", err)
		return []byte{}, err
	}

	raw, err := json.Marshal(&cookies)
	if err != nil {
		err = fmt.Errorf("failed to marshal cookies: %w", err)
		return []byte{}, err
	}

	return raw, nil
}

// SetCookies takes JSON content from the provided io.reader and sets cookies
// on the Scraper's browser.
//
// This is used in conjunction with `GetCookies` to carry-over the current
// browser state/session forward to subsequent scraper runs.
func (s *Scraper) SetCookies(raw io.Reader) error {
	buf := bytes.Buffer{}
	_, err := buf.ReadFrom(raw)
	if err != nil {
		err = fmt.Errorf("failed to read cookie reader: %w", err)
		return err
	}

	cookies := []*proto.NetworkCookie{}
	err = json.Unmarshal(buf.Bytes(), &cookies)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal cookies: %w", err)
		return err
	}

	s.browser.SetCookies(proto.CookiesToParams(cookies))
	return nil
}

// GetPurchasePrices returns current pricing for new contracts with DVC
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

	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Println("Error: ", err)
		}
		return prices, errs[0]
	}

	return prices, nil
}

// Close cleans up resources for the Scraper
func (s *Scraper) Close() error {
	err := s.cleanup()
	if err != nil {
		log.Println(err.Error())
	}

	return s.browser.Close()
}

func (s *Scraper) cleanup() error {
	cookieWriter, err := os.Create(cooieSessionFile)
	if err != nil {
		err = fmt.Errorf("failed to open session cookie file: %w", err)
		return err
	}
	defer cookieWriter.Close()

	cookieBytes, err := s.GetCookies()
	if err != nil {
		err = fmt.Errorf("failed to get cookies: %w", err)
		return err
	}

	_, err = cookieWriter.Write(cookieBytes)
	return err
}

// Login authenticates to gain access to protected parts of the DVC site
func (s *Scraper) Login() error {
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

	err = typeInput(frame, signInEmailSelector, s.email)
	if err != nil {
		err = fmt.Errorf("failed to input email address: %w", err)
		return err
	}

	err = typeInput(frame, signInPasswordSelector, s.password)
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
	return stealth.Page(s.browser)
}

func textOfElement(page elementable, selector string) (string, error) {
	elem, err := page.Element(selector)
	if err != nil {
		err = fmt.Errorf("failed to get element for text: %w", err)
		return "", err
	}

	return elem.Text()
}
