package dvcscraper

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const (
	signinURL = "https://disneyvacationclub.disney.go.com/sign-in/"

	dashboardCheckSelector = ".memberNewsAlert"
	signInBodySelector     = "body#registration_sign_in"
	signInEmailSelector    = ".field-username-email input"
	signInPasswordSelector = ".field-password input"
	signInSubmitSelector   = ".workflow-login .btn-submit"
	signInErrorSelector    = ".banner.login.message-error.message.state-active"
)

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

	wait := waitNavigation(page)
	err = click(frame, signInSubmitSelector)
	if err != nil {
		err = fmt.Errorf("failed to click to sign in: %w", err)
		return err
	}
	wait()

	err = rod.Try(func() {
		page.Timeout(10 * time.Second).MustElement(dashboardCheckSelector)
	})
	if errors.Is(err, context.DeadlineExceeded) {
		page.MustScreenshotFullPage("login-error.png")
		signInMsg, err := frame.Element(signInErrorSelector)
		if err != nil {
			err = fmt.Errorf("failed to get sign in error: %w (See login-error.png for details)", err)
			return err
		}
		text, _ := signInMsg.Text()
		err = fmt.Errorf("failed to login: '%s'. See login-error.png for more details.", text)
		return err
	}

	return err
}
