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
	signinURL            = "https://disneyvacationclub.disney.go.com/sign-in/"
	signinSuccessTimeout = 15 * time.Second

	dashboardCheckSelector = ".news-alert-header"
	signInIFrameSelector   = `iframe[id="disneyid-iframe"]`
	signInBodySelector     = "div#disneyid-wrapper"
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
	s.logger.Println("got page for auth")

	err = page.Navigate(signinURL)
	if err != nil {
		err = fmt.Errorf("failed to visit sign in page: %w", err)
		return err
	}
	s.logger.Println("navigated for auth")

	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  2560,
		Height: 1400,
	})
	if err != nil {
		err = fmt.Errorf("failed to set viewport: %w", err)
		return err
	}

	frame, err := getIFrame(page, signInIFrameSelector)
	if err != nil {
		err = fmt.Errorf("failed to get iframe page: %w", err)
		return err
	}
	s.logger.Println("got iframe for auth")

	err = typeInput(frame, signInEmailSelector, s.email)
	if err != nil {
		err = fmt.Errorf("failed to input email address: %w", err)
		return err
	}
	s.logger.Println("entered email for auth")

	err = typeInput(frame, signInPasswordSelector, s.password)
	if err != nil {
		err = fmt.Errorf("failed to input password: %w", err)
		return err
	}
	s.logger.Println("entered password for auth")

	wait := waitNavigation(page)
	err = s.click(frame, signInSubmitSelector)
	if err != nil {
		err = fmt.Errorf("failed to click to sign in: %w", err)
		return err
	}
	wait()
	s.logger.Println("clicked sign in for auth")

	s.logger.Println("waiting for sign in results")
	err = rod.Try(func() {
		s.logger.Println("checking for", dashboardCheckSelector)
		page.Timeout(signinSuccessTimeout).MustElement(dashboardCheckSelector)
	})
	if errors.Is(err, context.DeadlineExceeded) {
		filename := fmt.Sprintf("login-error-%s.png", time.Now().Format(time.RFC3339))
		page.MustScreenshotFullPage(filename)
		lErr := loginError{}
		signInMsg, err := frame.Element(signInErrorSelector)
		if err != nil {
			lErr.msg = fmt.Sprintf(
				"failed to get sign in error: %s (See %s for details)",
				err.Error(),
				filename,
			)
			return lErr
		}
		text, err := signInMsg.Text()
		if err != nil {
			lErr.msg = fmt.Sprintf("failed to get sign in message text after timeout: %s", err.Error())
			return lErr
		}
		lErr.msg = fmt.Sprintf("failed to login: '%s'. See %s for more details.", text, filename)
		lErr.certainlyFailed = true
		return lErr
	}

	return err
}

type loginError struct {
	msg             string
	certainlyFailed bool
}

func (l loginError) Error() string         { return l.msg }
func (l loginError) CertainlyFailed() bool { return l.certainlyFailed }
