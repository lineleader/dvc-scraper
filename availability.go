package dvcscraper

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

const (
	bookingPage = "https://disneyvacationclub.disney.go.com/booking/"
	calendarURL = "https://disneyvacationclub.disney.go.com/booking-api/api/v1/calendar-availability"

	calendarPickerMonthSelector = ".mobCoreDatepickerRange ul.carousel-wrapper li.carousel-slide"
	calendarPickerDaySelector   = "td[data-date='%s']"
	deluxeStudioButtonSelector  = "#mobBookingRoomType button[data-capacity='deluxe-studio']"
	closeTermsSelector          = "button#closeTermsOfUse"

	checkAvailabilityButtonSelector = "button#checkAvailabilityBtn"

	uiDateFormat = "01/02/2006"
	dateFormat   = "2006-01-02"
)

// AvailabilityOptions configure an availability request
type AvailabilityOptions struct {
	Resort   string    `json:"resort"`
	RoomType string    `json:"roomType"`
	Date     time.Time `json:"startDate"`
}

type AvailabilityResults struct {
	ResortCode   string `json:"resortCode"`
	RoomCode     string `json:"roomCode"`
	Availability []struct {
		Date   string `json:"date"`
		Rooms  int    `json:"rooms"`
		Points int    `json:"points"`
	} `json:"availability"`
}

type CalendarRequestBody struct {
	Resort     string    `json:"resort"`
	RoomType   string    `json:"roomType"`
	StartDate  string    `json:"startDate"`
	EndDate    string    `json:"endDate"`
	ParentID   *struct{} `json:"parentId"`
	Accessible bool      `json:"accessible"`
	IsModify   bool      `json:"isModify"`
}

type AvailabilityHandle struct {
	page *rod.Page
}

func (s *Scraper) NewAvailabilityHandle() (*AvailabilityHandle, error) {
	handle := AvailabilityHandle{}
	page, err := s.getPage()
	if err != nil {
		err = fmt.Errorf("failed to get page: %w", err)
		return &handle, err
	}
	s.logger.Println("got page for avail")

	handle.page = page

	err = s.AuthenticatedNavigate(bookingPage, closeTermsSelector)
	if err != nil {
		err = fmt.Errorf("failed to navigate to booking page: %w", err)
		return &handle, err
	}
	s.logger.Println("navigated to booking page for avail")

	err = s.click(page, closeTermsSelector)
	if err != nil {
		err = fmt.Errorf("failed to click close terms button: %w, moving on", err)
		s.logger.Println(err.Error())
	} else {
		s.logger.Println("Clicked close terms button")
	}

	startDate, endDate := bookingDates()
	startSelector := calendarPickerMonthSelector + " " + fmt.Sprintf(calendarPickerDaySelector, startDate)
	err = s.click(page, startSelector)
	if err != nil {
		err = fmt.Errorf("failed to click start date (%s): %w", startDate, err)
		return &handle, err
	}
	s.logger.Println("Clicked start date")

	endDateSelector := calendarPickerMonthSelector + " " + fmt.Sprintf(calendarPickerDaySelector, endDate)
	err = s.click(page, endDateSelector)
	if err != nil {
		err = fmt.Errorf("failed to click end date (%s): %w", endDate, err)
		return &handle, err
	}
	s.logger.Println("Clicked end date")

	err = s.click(page, deluxeStudioButtonSelector)
	if err != nil {
		err = fmt.Errorf("failed to click deluxe studio button: %w", err)
		return &handle, err
	}
	s.logger.Println("Clicked studio button")

	err = s.click(page, checkAvailabilityButtonSelector)
	if err != nil {
		err = fmt.Errorf("failed to click check availability button: %w", err)
		return &handle, err
	}
	s.logger.Println("Clicked check availability button")

	err = page.WaitLoad()
	if err != nil {
		err = fmt.Errorf("failed to wait for search page to load: %w", err)
		return &handle, err
	}
	s.logger.Println("Waited loading")

	return &handle, nil
}

func (h *AvailabilityHandle) GetAvailability(opts AvailabilityOptions) (AvailabilityResults, error) {
	results := AvailabilityResults{}
	page := h.page

	start, end := startEnd(opts.Date)
	body := CalendarRequestBody{
		Resort:    opts.Resort,
		RoomType:  opts.RoomType,
		StartDate: start.Format(dateFormat),
		EndDate:   end.Format(dateFormat),
	}

	obj, err := page.Evaluate(&rod.EvalOptions{
		AwaitPromise: true,
		ByValue:      true,
		UserGesture:  true,
		ThisObj:      nil,
		JS:           getAvailJS,
		JSArgs: []interface{}{
			calendarURL,
			body,
		},
	})
	if err != nil {
		err = fmt.Errorf("failed to Evaluate: %w", err)
		return results, err
	}

	err = json.Unmarshal([]byte(obj.Value.JSON("", "")), &results)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal results: %w -- %s", err, obj.Value.JSON("", ""))
		return results, err
	}

	return results, nil
}

func bookingDates() (string, string) {
	initial := time.Now().AddDate(0, 7, 0)
	loc := initial.Location()
	y, m, _ := initial.Date()
	startOfMonth := time.Date(y, m, 1, 0, 0, 0, 0, loc)
	startDate := startOfMonth.Format(uiDateFormat)
	endDate := startOfMonth.AddDate(0, 0, 5).Format(uiDateFormat)
	return startDate, endDate
}

func startEnd(in time.Time) (time.Time, time.Time) {
	loc := in.Location()
	y, m, _ := in.Date()
	startOfMonth := time.Date(y, m, 1, 0, 0, 0, 0, loc)
	startDate := startOfMonth
	endDate := startOfMonth.AddDate(0, 1, -1)

	if startDate.Before(time.Now()) {
		startDate = time.Now()
	}

	if endDate.Month() == time.Now().AddDate(0, 11, 0).Month() {
		endDate = time.Now().AddDate(0, 11, 6)
	}

	return startDate, endDate
}

const getAvailJS = `(url, body) => {
	const controller = new AbortController();
	const timeoutId = setTimeout(() => controller.abort(), 5000)
	return fetch(url, {
		signal: controller.signal,
		method: "POST",
		headers: {
			Accept: "application/json, text/plain, */*",
			"Accept-Language": "en-US,en;q=0.5",
			"Content-Type": "application/json;charset=utf-8",
			ADRUM: "isAjax:true",
			Pragma: "no-cache",
			"Cache-Control": "no-cache",
		},
		body: JSON.stringify(body),
	}).then(r => r.json()).catch(error => error.message)
}
`
