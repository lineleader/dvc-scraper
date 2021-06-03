package dvcscraper

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const (
	bookingPage = "https://disneyvacationclub.disney.go.com/booking/"
	calendarURL = "https://disneyvacationclub.disney.go.com/booking-api/api/v1/calendar-availability"

	calendarPickerMonthSelector = ".mobCoreDatepickerRange ul.carousel-wrapper li.carousel-slide"
	calendarPickerDaySelector   = "td[data-date='%s']"
	deluxeStudioButtonSelector  = "#mobBookingRoomType button[data-capacity='deluxe-studio']"

	checkAvailabilityButtonSelector = "button#checkAvailabilityBtn"

	uiDateFormat = "01/02/2006"
	dateFormat   = "2006-01-02"
)

type AvailabilityOptions struct {
	Resort    string `json:"resort"`
	RoomType  string `json:"roomType"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
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

	handle.page = page

	err = page.Navigate(bookingPage)
	if err != nil {
		err = fmt.Errorf("failed to navigate to booking page: %w", err)
		return &handle, err
	}

	page.MustScreenshot("booking-page.png")

	startDate, endDate := bookingDates()
	startSelector := calendarPickerMonthSelector + " " + fmt.Sprintf(calendarPickerDaySelector, startDate)
	log.Println("start", startSelector)
	err = click(page, startSelector)
	if err != nil {
		err = fmt.Errorf("failed to click start date (%s): %w", startDate, err)
		return &handle, err
	}

	log.Println("Selected start date")

	endDateSelector := calendarPickerMonthSelector + " " + fmt.Sprintf(calendarPickerDaySelector, endDate)
	log.Println("end", endDateSelector)
	err = click(page, endDateSelector)
	if err != nil {
		err = fmt.Errorf("failed to click end date (%s): %w", endDate, err)
		return &handle, err
	}

	log.Println("Selected end date")

	err = click(page, deluxeStudioButtonSelector)
	if err != nil {
		err = fmt.Errorf("failed to click deluxe studio button: %w", err)
		return &handle, err
	}

	log.Println("Selected deluxe studio")

	button, err := page.Element(checkAvailabilityButtonSelector)
	if err != nil {
		err = fmt.Errorf("failed to get check availability button element: %w", err)
		return &handle, err
	}

	err = button.ScrollIntoView()
	if err != nil {
		err = fmt.Errorf("failed to scroll availability button into view: %w", err)
		return &handle, err
	}
	page.MustScreenshot("filled-form.png")

	err = button.Click(proto.InputMouseButtonLeft)
	if err != nil {
		err = fmt.Errorf("failed to click availability button: %w", err)
		return &handle, err
	}
	log.Println("Clicked check availability button")

	err = page.WaitLoad()
	if err != nil {
		err = fmt.Errorf("failed to wait for search page to load: %w", err)
		return &handle, err
	}

	return &handle, nil
}

func (h *AvailabilityHandle) GetAvailability(opts AvailabilityOptions) (AvailabilityResults, error) {
	results := AvailabilityResults{}
	page := h.page

	log.Println("opts:", opts)
	body := CalendarRequestBody{
		Resort:    opts.Resort,
		RoomType:  opts.RoomType,
		StartDate: opts.StartDate,
		EndDate:   opts.EndDate,
	}

	obj, err := page.Evaluate(&rod.EvalOptions{
		AwaitPromise: true,
		ByValue:      true,
		UserGesture:  false,
		ThisObj:      nil,
		JS:           getAvailJS,
		JSArgs: []interface{}{
			calendarURL,
			body,
		},
	})
	if err != nil {
		err = fmt.Errorf("failed to Evaluate: %w", err)
		log.Fatal(err)
	}
	log.Println(obj.Value.Get("resortCode").Str())
	log.Println(obj.Value.Str())
	log.Println(obj.Value.JSON("", "  "))

	err = json.Unmarshal([]byte(obj.Value.JSON("", "")), &results)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal results: %w", err)
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

const getAvailJS = `(url, body) => fetch(url,
{
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
}).then(r => r.json())`