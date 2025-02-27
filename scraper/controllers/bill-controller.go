// Package controllers is responsible for retrieving data and updating to database
package controllers

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ledongthuc/pdf"
	"unicode"

	//pdf "github.com/ledongthuc/pdf"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/queue"
	"pranavputta.me/oddysey/scraper/db"
	"pranavputta.me/oddysey/scraper/models"
)

var _categoriesJson map[string]string

type ScrapeHandler struct {
	mu            sync.Mutex
	count         int
	bills         []models.Bill
	notifications []models.Notification
}

// getCategoriesJson converts the human readable map and inverses key
// 					 value pairs for the map to be legible by the computer
func getCategoriesJson() map[string]string {
	if len(_categoriesJson) == 0 {
		// load json file into an empty map
		jsonFile, err := os.Open("categories.json")
		if err != nil {
			fmt.Println(err)
		}
		byteValue, _ := ioutil.ReadAll(jsonFile)
		var data map[string]interface{}
		err = json.Unmarshal(byteValue, &data)
		if err != nil {
			fmt.Println(err)
		}
		err = jsonFile.Close()
		if err != nil {
			fmt.Println(err)
		}

		// initialize map
		_categoriesJson = make(map[string]string)
		// loop through each key in map
		for k, v := range data {
			// type assert that v is a string array of committeeIDs
			for _, f := range v.([]interface{}) {
				_categoriesJson[strconv.Itoa(int(f.(float64)))] = k
			}
		}
	}
	return _categoriesJson
}

// BillCallback is a function to be called when bill information is retrieved
type BillCallback func(models.Bill, models.Notification, bool, error)

// RefreshBills
func RefreshBills(url string, ga string, sessionID string) {
	// initialize systems
	db.Initialize()
	getCategoriesJson()

	// initialize synchronized mutex
	handler := new(ScrapeHandler)
	handler.bills = make([]models.Bill, 0)
	handler.notifications = make([]models.Notification, 0)

	err := ScrapeBills(url, ga, sessionID, func(b models.Bill, n models.Notification, shouldNotify bool, err error) {
		handler.mu.Lock()
		defer handler.mu.Unlock()

		// update items
		handler.count++
		handler.bills = append(handler.bills, b)

		// check if notification needs to be sent
		if shouldNotify {
			handler.notifications = append(handler.notifications, n)
		}

		fmt.Printf("Done: Bill #%d, (%d)\n", b.Metadata.Number, handler.count)
	})
	if err != nil {
		panic(err)
	}

	for _, b := range handler.bills {
		db.InsertBill(b)
	}
	fmt.Println("starting upload to database...")
	db.Finish()

	fmt.Println("sending notifications...")
	notificationMap := map[string][]models.Notification{"notifications": handler.notifications}
	val, _ := json.MarshalIndent(notificationMap, "", "\t")
	fmt.Println(notificationMap)

	resp, err := http.Post(NotificationURL, "application/json", bytes.NewBuffer(val))

	if err != nil {
		panic(err)
	}

	fmt.Printf("sent notifications: %d\n", resp.StatusCode)

	fmt.Println("done!")
}

// ScrapeBills retrieves all bill data given the url of a list of bills from http://ilga.gov/
func ScrapeBills(url string, ga string, sessionID string, callback BillCallback) error {
	// bill details collector
	billCollector := colly.NewCollector(colly.Async(true), colly.DetectCharset(), colly.AllowURLRevisit())
	billCollector.SetRequestTimeout(20 * time.Second)

	// setup collectors
	err := billCollector.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 10})
	if err != nil {
		panic(err)
	}
	billCollector.OnHTML("body", func(e *colly.HTMLElement) {
		onBillDetailsResponse(e, callback)
	})
	billCollector.OnError(func(response *colly.Response, err error) {
		err = response.Request.Retry()
		if err != nil {
			panic(err)
		}
	})

	// get bills document from provided url
	url = fmt.Sprintf(url, ga, sessionID)
	billsDoc, err := goquery.NewDocument(url)
	if err != nil {
		return errors.New("couldn't scrape")
	}

	q, _ := queue.New(1, &queue.InMemoryQueueStorage{MaxSize: 100000})

	// go through each link on the page
	billsDoc.Find("li").Each(func(i int, el *goquery.Selection) {
		// access each bill link and visit
		link, exists := el.Find("a").Attr("href")
		if !exists {
			return
		}

		// get absolute url
		link, err := absoluteURL(link)
		if err != nil {
			panic(err)
			return
		}
		q.AddURL(link)
	})
	q.Run(billCollector)
	billCollector.Wait()

	return nil
}

// callback for when bill details collector respond
func onBillDetailsResponse(e *colly.HTMLElement, callback BillCallback) {
	doc := e.DOM
	shouldUpdateActions, shouldUpdateAll := false, false
	var notification models.Notification
	var shouldNotify = false

	// get metadata to retrieve from database
	md, err := buildBillMetadata(e)

	if err != nil {
		fmt.Println(e.Request.URL.String(), " : ", err)
		return
	}

	// get row from database
	bill, err := db.GetBill(md)
	hash := ""

	if err != nil {
		shouldUpdateAll = true
	}
	// bill actions
	hash = hashActions(doc)
	shouldUpdateActions = bill.ActionsHash != hash

	// always update bill metadata, title, summary, and sponsors
	// bill metadata
	bill.Metadata = md

	// bill title
	bill.Title = doc.Find(`span:contains("Short Description") ~ span.content`).First().Text()

	// bill summary
	bill.ShortSummary = doc.Find(`span:contains("Synopsis") ~ span.content`).First().Text()
	bill.FullSummary = doc.Find(`span:contains("Synopsis") ~ span.content`).First().NextAllFiltered("span.content").Text()

	// bill sponsors
	bill.SponsorIDs, bill.HousePrimarySponsor, bill.SenatePrimarySponsor, bill.ChiefSponsor = buildSponsors(doc)

	// if actions are different, update actions as well and decide whether to update votes and full text
	if shouldUpdateActions || shouldUpdateAll {
		// bill actions
		actionsTemp, category, committee := buildActions(doc)
		updateText, updateVotes := checkActionsForUpdates(bill.Actions, actionsTemp)
		notification, shouldNotify, bill.Viewable = checkNotification(bill.Actions, actionsTemp, bill.Metadata)

		// update actions into bill doc
		bill.Actions = actionsTemp
		bill.Category = category
		bill.ActionsHash = hash
		bill.CommitteeID = committee
		if len(bill.Actions) > 0 {
			bill.Created = bill.Actions[0].Date
		} else {
			bill.Created = 0
		}

		if updateText || shouldUpdateAll {
			// create the url for full text
			fullTextURL := fmt.Sprintf("http://www.ilga.gov/legislation/%d/%s/%d00%s%04d.htm",
				bill.Metadata.Assembly, bill.Metadata.Chamber, bill.Metadata.Assembly, bill.Metadata.Chamber, bill.Metadata.Number)

			newDoc, err := goquery.NewDocument(fullTextURL)
			fullText := ""
			if err == nil {
				fullText = buildFullText(newDoc.Find("html"))
			}

			bill.BillText.URL = fullTextURL
			bill.BillText.FullText = fullText
		}

		if updateVotes || shouldUpdateAll {
			bill.VoteEvents = buildVotes(doc)
		}
	}
	callback(bill, notification, shouldNotify, nil)
}

// hash the actions table for comparison
func hashActions(doc *goquery.Selection) string {
	// find actions table
	actions := doc.Find(`a[name="actions"] ~ table`).First().Text()

	// hash the html and compare
	hash := md5.Sum([]byte(actions))
	return hex.EncodeToString(hash[:])
}

// build bill metadata
func buildBillMetadata(e *colly.HTMLElement) (models.BillMetadata, error) {
	var metadata models.BillMetadata

	url := e.Request.URL.String()

	// bill number details
	r := regexp.MustCompile("DocNum=([0-9]+)")
	tmp := strings.Split(r.FindString(url), "=")
	var billNum int64

	if len(tmp) > 1 {
		tmp1, err := strconv.ParseInt(tmp[1], 0, 64)
		if err != nil {
			return metadata, errors.New("couldn't get bill number")
		}
		billNum = tmp1
	} else {
		return metadata, errors.New("couldn't get bill number")
	}

	// bill chamber details
	r = regexp.MustCompile("DocTypeID=([A-Za-z]+)")
	tmp = strings.Split(r.FindString(url), "=")
	billChamber := ""

	if len(tmp) > 1 {
		billChamber = tmp[1]
	} else {
		return metadata, errors.New("couldn't get bill number")
	}

	// bill general assembly
	r = regexp.MustCompile("GA=([0-9]+)")
	tmp = strings.Split(r.FindString(url), "=")
	var billGA int64

	if len(tmp) > 1 {
		tmp1, err := strconv.ParseInt(tmp[1], 0, 64)
		if err != nil {
			return metadata, errors.New("couldn't get bill number")
		}
		billGA = tmp1
	} else {
		return metadata, errors.New("couldn't get bill number")
	}

	metadata = models.BillMetadata{
		Assembly: billGA,
		Chamber:  billChamber,
		Number:   billNum,
		URL:      url,
	}

	return metadata, nil
}

// generate the list of sponsors for this bill
func buildSponsors(doc *goquery.Selection) (sponsors []int, housePrimaryID int, senatePrimaryID int, chiefID int) {
	housePrimaryID, senatePrimaryID, chiefID = -1, -1, -1
	housePrimarySelected, senatePrimarySelected := false, false

	// loop through each sponsor : query: "a.content"
	doc.Find(`a.content`).Each(func(i int, s *goquery.Selection) {
		sponsorID := -1
		href, hrefExists := s.Attr("href")

		// only if href exists should further information be captured
		if hrefExists {
			// extract the member id from href
			r := regexp.MustCompile("MemberID=([0-9]+)")
			tmp := strings.Split(r.FindString(href), "=")
			if len(tmp) > 1 {
				id, err := strconv.Atoi(tmp[1])
				if err == nil {
					sponsorID = id
				}
			}

			if i == 0 {
				chiefID = sponsorID
			}

			// determine chamber type : house or senate
			if strings.Contains(href, "house") {
				// if no house primary is taken yet, assign
				if !housePrimarySelected {
					housePrimarySelected = false
					housePrimaryID = sponsorID
				}
			} else if strings.Contains(href, "senate") {
				// if no senate primary is taken yet, assign
				if !senatePrimarySelected {
					senatePrimarySelected = true
					senatePrimaryID = sponsorID
				}
			}
		}
		// append into list
		sponsors = append(sponsors, sponsorID)
	})

	return sponsors, housePrimaryID, senatePrimaryID, chiefID
}

// generate the list of actions for this bill
func buildActions(doc *goquery.Selection) (actions []models.BillAction, category models.BillCategory, committee string) {
	actionsTable := doc.Find(`a[name="actions"] ~ table`).First().Find("tr")
	category = models.DNE
	committee = ""
	actionsTable.Each(func(i int, s *goquery.Selection) {
		td := s.Find(`td.content`)
		// if not heading row
		if td.Size() == 3 {
			// date of action
			date, err := time.Parse(`1/2/2006`, strings.TrimSpace(td.Eq(0).Text()))
			// convert date into utc milliseconds
			var millis int64 = -1
			if err == nil {
				millis = date.UTC().UnixNano() / 1000000
			}
			// legislative body of action
			chamber := td.Eq(1).Text()
			// action text
			action := td.Eq(2).Text()
			// tag
			tag := tagAction(action)

			// check if action points to category
			if tag == models.Assigned {
				category, committee = identifyBillCategory(td.Eq(2))
			}
			// append to list
			actions = append(actions, models.BillAction{
				Date:        millis,
				Chamber:     chamber,
				Description: action,
				Tag:         tag})
		}
	})
	// TODO: match sponsors with actions in the future
	return actions, category, committee
}

// scrape data from full bill text html
func buildFullText(doc *goquery.Selection) string {
	fullText := ""
	// find all tr elements with a td element with xsl class
	doc.Find("tr").Has(".xsl").Each(func(_ int, sel *goquery.Selection) {
		// if the number column exists
		if len(sel.Find("td.number").Text()) > 0 {
			fullText += sel.Find("td.xsl").Text()
		}
	})

	return fullText
}

func buildVotes(doc *goquery.Selection) (votes []models.BillVoteEvent) {
	// 1. get table of links for voting
	voteLink, exists := doc.Find(`a.legislinks:contains("Votes")`).Attr("href")
	if !exists {
		return votes
	}
	voteLink = fmt.Sprintf("%s/legislation/%s", RootURL, voteLink)

	voteDoc, err := goquery.NewDocument(voteLink)
	if err != nil {
		return votes
	}

	html := voteDoc.Find("html")
	td := html.Find(`td.whiteheading:contains("Voting Record")`).Last()
	trs := td.Parent().Siblings()

	for r := 0; r < trs.Size(); r++ {
		link, exists := trs.Eq(r).Find("a").Attr("href")
		link = fmt.Sprintf("%s%s", RootURL, link)
		if exists {
			chamber := trs.Eq(r).Find("td").Eq(1).Text()
			event, err := buildVotingEvent(link, chamber)
			if err == nil {
				votes = append(votes, event)
			}
		}
	}
	return votes
}

func buildVotingEvent(votingURL string, chamber string) (models.BillVoteEvent, error) {
	var event models.BillVoteEvent
	// retrieve voting pdf
	resp, err := http.Get(votingURL)
	if err != nil {
		return event, err
	}

	defer resp.Body.Close()

	// read into buffer stream
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return event, nil
	}
	reader := bytes.NewReader(body)

	// convert into pdf parser
	parser, err := pdf.NewReader(reader, reader.Size())
	if err != nil {
		panic(err)
	}

	// pull pdf parser into text stream
	b, err := parser.GetPlainText()
	var buf bytes.Buffer
	buf.ReadFrom(b)
	//text := buf.String()

	// convert into bill vote event

	votes := make(map[string]string)

	for page := 1; page <= parser.NumPage(); page++ {
		rows, err := parser.Page(page).GetTextByRow()
		if err != nil {
			panic(err)
		}

		var evts = map[string]int{
			"Y":  0,
			"N":  1,
			"E":  2,
			"NV": 3,
			"P":  4,
		}
		for _, row := range rows {
			for _, word := range row.Content {
				fields := strings.Fields(word.S)

				for i := 0; i < len(fields); i += 2 {
					v := strings.Trim(fields[i], " ")
					if _, ok := evts[v]; ok {
						//do something here
						name := strings.Trim(fields[i+1], " ")

						if unicode.IsLetter(rune(name[0])) {
							votes[name] = v
						}
					}
				}
			}
		}
	}
	return models.BillVoteEvent{Votes: votes, Chamber: strings.ToLower(chamber)}, nil
}

// returns tag associated with action description
func tagAction(action string) models.Tag {
	if strings.Contains(action, "Assigned to") {
		return models.Assigned
	} else if strings.Contains(action, "Effective Date") {
		return models.EffectiveDate
	} else if strings.Contains(action, "Arrived in") {
		if strings.Contains(action, "House") {
			return models.ArrivalInHouse
		} else if strings.Contains(action, "Senate") {
			return models.ArrivalInSenate
		}
	} else if strings.Contains(action, "Added as") && strings.Contains(action, "Sponsor") {
		return models.CoSponsor
	} else if strings.Contains(action, "Placed on Calendar Order of 3rd Reading") {
		return models.ThirdReadingVote
	} else if strings.Contains(action, "Do Pass") {
		return models.CommitteeDebate
	} else if strings.Contains(action, "Alternate Chief") {
		return models.SponsorRemoved
	} else if strings.Contains(action, "Fiscal Note Requested") {
		return models.FiscalRequest
	} else if strings.Contains(action, "Passed Both Houses") {
		return models.DualPassed
	} else if strings.Contains(action, "Sent") && strings.Contains(action, "Governor") {
		return models.SentToGovernor
	} else if strings.Contains(action, "Governor Approved") {
		return models.GovernorApproved
	} else if strings.Contains(action, "Public Act") {
		return models.PublicAct
	} else if strings.Contains(action, "Third Reading") {
		if strings.Contains(action, "Passed") {
			return models.BillVotePass
		} else if strings.Contains(action, "Failed") {
			return models.BillVoteFail
		}
	} else if strings.Contains(action, "Amendment") && strings.Contains(action, "Adopted") {
		return models.Amended
	} else if strings.Contains(action, "First Reading") {
		return models.FirstReading
	} else if strings.Contains(action, "Second Reading") {
		return models.SecondReading
	}
	return models.Other
}

// identifyBillCategory determines the bill category according to the committee "assigned to"
func identifyBillCategory(action *goquery.Selection) (models.BillCategory, string) {
	href, exists := action.Find(`a`).Attr("href")
	if !exists {
		return models.DNE, ""
	}
	// get committeeID using regex
	r := regexp.MustCompile("committeeID=([0-9]+)")
	tmp := strings.Split(r.FindString(href), "=")
	if len(tmp) <= 1 {
		return models.DNE, ""
	}

	// create type assertion that
	cat, ok := getCategoriesJson()[tmp[1]]
	if !ok {
		return models.DNE, tmp[1]
	}
	return models.BillCategory(cat), tmp[1]
}

// check action labels for indication that a vote or amendment may have happened
func checkActionsForUpdates(old []models.BillAction, new []models.BillAction) (updateText bool, updateVotes bool) {
	start := len(old)
	updateText, updateVotes = false, false

	// loop through each new element
	for i := start; i < len(new); i++ {
		if new[i].Tag == models.Amended {
			updateText = true
		}
		if new[i].Tag == models.BillVotePass || new[i].Tag == models.BillVoteFail {
			updateVotes = false
		}
	}

	return updateText, updateVotes
}

// checks for notification
func checkNotification(old []models.BillAction, new []models.BillAction, bill models.BillMetadata) (notification models.Notification, shouldNotify bool, viewable bool) {
	// search actions list for particular tag
	search := func(i int, tag models.Tag, actions []models.BillAction) (int, models.BillAction, error) {
		var action models.BillAction
		for ; i < len(actions); i++ {
			if actions[i].Tag == tag {
				return i, actions[i], nil
			}
		}
		return i, action, errors.New("end of list")
	}

	// find the latest tag
	latest := func(actions []models.BillAction) (models.BillAction, bool, error) {
		i := 0
		var latestAction models.BillAction
		var err error
		tags := [8]models.Tag{models.FirstReading, models.SecondReading, models.BillVotePass, models.FirstReading, models.SecondReading, models.BillVotePass, models.SentToGovernor, models.PublicAct}
		var ct = 0
		for j, tag := range tags {
			ct = j
			var tmp models.BillAction
			i, tmp, err = search(i, tag, actions)

			if err != nil {
				if j == 0 {
					return latestAction, false, errors.New("no latest action")
				}
				break
			}
			latestAction = tmp
		}

		return latestAction, ct >= 2, nil
	}

	loAction, _, loErr := latest(old)
	lnAction, viewable, lnErr := latest(new)

	var notif models.Notification

	if (loErr != nil && lnErr == nil) || (loErr == nil && lnErr == nil && (loAction.Tag != lnAction.Tag || loAction.Chamber != lnAction.Chamber)) {
		// map action to string
		var sb strings.Builder
		chamber := "SB"
		if lnAction.Chamber != "Senate" {
			chamber = "HB"
		}
		sb.WriteString("Bill ")
		sb.WriteString(chamber)
		sb.WriteString(fmt.Sprintf("%d", bill.Number))
		sb.WriteString(" update: ")
		tag := lnAction.Tag
		if tag == models.FirstReading {
			sb.WriteString(fmt.Sprintf("Arrived in %s", lnAction.Chamber))
		} else if tag == models.SecondReading {
			sb.WriteString(fmt.Sprintf("Debating in %s", lnAction.Chamber))
		} else if tag == models.BillVotePass {
			sb.WriteString(fmt.Sprintf("Passed in %s", lnAction.Chamber))
		} else if tag == models.SentToGovernor {
			sb.WriteString(fmt.Sprintf("Passed both chambers and waiting for governor"))
		} else if tag == models.PublicAct {
			sb.WriteString(fmt.Sprintf("Bill passed into law!"))
		}

		// timeline is different, create a notification
		notif = models.Notification{
			BillInfo: bill,
			Text:     sb.String(),
		}
		return notif, true, viewable
	}
	return notif, false, false
}

// takes url and prepends root url
func absoluteURL(href string) (string, error) {
	if strings.HasPrefix(href, RootURL) {
		return href, nil
	} else if strings.HasPrefix(href, "/") {
		return fmt.Sprintf("%s%s", RootURL, href), nil
	} else {
		return href, errors.New("couldn't derive absolute url")
	}
}
