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
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

const (
	signinURL = "https://disneyvacationclub.disney.go.com/sign-in/"

	dashboardCheckSelector = ".memberNewsAlert"
	signInEmailSelector    = ".field-username-email input"
	signInPasswordSelector = ".field-password input"
	signInSubmitSelector   = ".workflow-login .btn-submit"

	cooieSessionFile = ".dvcscraper-session.json"
)

// Scraper provides authenticated access to the DVC website to scrape data easily
type Scraper struct {
	email    string
	password string

	logger *log.Logger

	browser *rod.Browser
	page    *rod.Page
}

type elementable interface {
	Element(string) (*rod.Element, error)
}

// New returns a Scraper ready to roll
func New(email, password string) (Scraper, error) {
	scraper := Scraper{
		email:    email,
		password: password,

		logger: log.Default(),
	}

	scraper.browser = rod.New()
	// scraper.browser.ServeMonitor(":9777")
	err := scraper.browser.Connect()
	if err != nil {
		err = fmt.Errorf("failed to connect to browser: %w", err)
		return scraper, err
	}
	err = scraper.readCookies()
	if err != nil {
		err = fmt.Errorf("failed to read cookies: %w", err)
	}
	return scraper, err
}

// NewWithBinary returns a new Scraper with the provided browser binary launched
func NewWithBinary(email, password, binpath string) (Scraper, error) {
	scraper := Scraper{
		email:    email,
		password: password,

		logger: log.Default(),
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
	if err != nil {
		err = fmt.Errorf("failed to connect to browser: %w", err)
		return scraper, err
	}
	err = scraper.readCookies()
	if err != nil {
		err = fmt.Errorf("failed to read cookies: %w", err)
	}
	return scraper, err
}

func (s *Scraper) SetLogger(logger *log.Logger) {
	s.logger = logger
}

func (s *Scraper) readCookies() error {
	_, err := os.Stat(cooieSessionFile)
	if os.IsNotExist(err) {
		// no previous session; continue
		return nil
	} else if err != nil {
		err = fmt.Errorf("failed to check for session file: %w", err)
		return err
	}

	cookieReader, err := os.Open(cooieSessionFile)
	if err != nil {
		err = fmt.Errorf("failed to read cookie session file: %w", err)
		return err
	}
	defer cookieReader.Close()

	err = s.SetCookies(cookieReader)
	if err != nil {
		err = fmt.Errorf("failed to set cookies: %w", err)
		return err
	}

	return nil
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

// Close cleans up resources for the Scraper
func (s *Scraper) Close() error {
	err := s.cleanup()
	if err != nil {
		s.logger.Println(err.Error())
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

	return err
}

func (s *Scraper) AuthenticatedNavigate(url string) error {
	page, err := s.getPage()
	if err != nil {
		err = fmt.Errorf("failed to get page for navigation: %w", err)
		return err
	}

	err = page.Navigate(url)
	if err != nil {
		err = fmt.Errorf("failed to navigate to '%s': %w", url, err)
		return err
	}

	notLoggedIn, err := onPage(page, "body#registration_sign_in")
	if err != nil {
		err = fmt.Errorf("failed to check if logged in: %w", err)
		return err
	}

	if notLoggedIn {
		s.logger.Println("Need to re-auth")
		err = s.Login()
		if err != nil {
			return err
		}
	}

	err = page.Navigate(url)
	if err != nil {
		err = fmt.Errorf("failed to navigate to '%s' after login: %w", url, err)
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
	var err error
	if s.page == nil {
		s.page, err = stealth.Page(s.browser)
		if err != nil {
			return s.page, err
		}
	}

	return s.page, nil
}

func textOfElement(page elementable, selector string) (string, error) {
	elem, err := page.Element(selector)
	if err != nil {
		err = fmt.Errorf("failed to get element for text: %w", err)
		return "", err
	}

	return elem.Text()
}

func onPage(page *rod.Page, selector string) (bool, error) {
	err := rod.Try(func() {
		page.Timeout(2 * time.Second).MustElement(selector)
	})
	if errors.Is(err, context.DeadlineExceeded) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}
