package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

type elementable interface {
	Element(string) (*rod.Element, error)
}

func main() {
	browser := rod.New()
	defer browser.Close()
	// browser.ServeMonitor(":9777")

	err := browser.Connect()
	if err != nil {
		err = fmt.Errorf("failed to connect to browser: %w", err)
		log.Fatal(err)
	}

	doLoginCheck(browser)

	cookies, err := browser.GetCookies()
	if err != nil {
		err = fmt.Errorf("failed to get cookies from browser: %w", err)
		log.Fatal(err)
	}

	raw, err := json.Marshal(&cookies)
	if err != nil {
		err = fmt.Errorf("failed to marshal cookies: %w", err)
		log.Fatal(err)
	}

	fmt.Println("\n\nsecond browser")
	secondBrowser := rod.New()
	defer secondBrowser.Close()
	err = secondBrowser.Connect()
	if err != nil {
		err = fmt.Errorf("failed to connect second browser: %w", err)
		log.Fatal(err)
	}

	ckies := []*proto.NetworkCookie{}
	err = json.Unmarshal(raw, &ckies)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal cookies: %w", err)
		log.Fatal(err)
	}

	secondBrowser.SetCookies(proto.CookiesToParams(ckies))

	doLoginCheck(secondBrowser)

	fmt.Println("Done")
}

func doLoginCheck(browser *rod.Browser) {
	page, err := stealth.Page(browser)
	if err != nil {
		err = fmt.Errorf("failed to get new page: %w", err)
		log.Fatal(err)
	}

	err = page.Navigate("https://my.growthtools.com/action-guides")
	if err != nil {
		err = fmt.Errorf("failed to navigate to action-guides: %w", err)
		log.Fatal(err)
	}

	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  2560,
		Height: 1400,
	})
	if err != nil {
		err = fmt.Errorf("failed to set viewport: %w", err)
		log.Fatal(err)
	}

	isOnPage, err := onPage(page, "section.page-header")
	if isOnPage {
		fmt.Println("logged in!")
		return
	} else {
		fmt.Println("logging in...")
		form, err := page.Element(".signin form")
		if err != nil {
			err = fmt.Errorf("failed to get signin form: %w", err)
			log.Fatal(err)
		}

		email := "authtest@example.com"
		password := "password"
		typeInput(form, "input[name=Email]", email)
		typeInput(form, "input[name=Password]", password)
		click(form, "input[type=Submit]")
	}

	isOnPage, err = onPage(page, "section.page-header")
	if isOnPage {
		fmt.Println("logged in!")
	} else {
		log.Fatal("no dice")
	}

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

func click(page elementable, selector string) error {
	clickElem, err := page.Element(selector)
	if err != nil {
		err = fmt.Errorf("failed to get element for click: %w", err)
		return err
	}

	return clickElem.Click(proto.InputMouseButtonLeft)
}

func typeInput(page elementable, selector, text string) error {
	inputElem, err := page.Element(selector)
	if err != nil {
		err = fmt.Errorf("failed to get element for input: %w", err)
		return err
	}

	return inputElem.Input(text)
}
