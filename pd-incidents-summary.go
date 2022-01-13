package main

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
)

var (
	serviceIDs []string
	authtoken  string
	since      string
	until      string
	client     *pagerduty.Client
)

type AlertDetails struct {
	name          string
	occuranceTime string
	TTA           float64
	TTR           float64
}

type AlertAnalytics struct {
	name           string
	occuranceTimes []string
	count          int
	MTTA           float64
	MTTR           float64
	offHourCount   int
	weekendCount   int
}

type AlertsSummary struct {
	totalCount          int
	MeanMTTR            float64
	MeanMTTA            float64
	totalOffHourCount   int
	totalWeekendCount   int
	offHourPercent      float64
	weekendPercent      float64
	totalOffHourPercent float64
}

func main() {

	var p = fmt.Println

	authtoken = "<YOUR PD TOKEN HERE>"
	serviceIDs = []string{"<SERVICE ID>"}
	since = "<SINCE TS, eg:2021-11-30T18:25:39Z>"
	until = "<UNTIL TS, eg:2021-12-05T18:25:39Z>"
	client = pagerduty.NewClient(authtoken)

	http.HandleFunc("/alertDetails", getDetails)
	http.HandleFunc("/alertAnalytics", getAnalytics)
	http.HandleFunc("/alertSummary", getSummary)
	http.ListenAndServe(":8080", nil)

	alertDetails := getAlertDetails()
	alertAnalytics := getAlertAnalytics(alertDetails)
	for k := range alertAnalytics {
		p(alertAnalytics[k].name, alertAnalytics[k].count, alertAnalytics[k].offHourCount, alertAnalytics[k].weekendCount, alertAnalytics[k].occuranceTimes, math.Round((alertAnalytics[k].MTTA/60)*100)/100, math.Round((alertAnalytics[k].MTTR/60)*100)/100)
	}
	p(getAlertsSummary(alertAnalytics))

}

func getDetails(w http.ResponseWriter, req *http.Request) {
	if t1, ok := req.URL.Query()["since"]; ok {
		since = t1[0]
		if t2, ok := req.URL.Query()["until"]; ok {
			until = t2[0]
		}
	}
	if ID, ok := req.URL.Query()["serviceID"]; ok {
		serviceIDs = ID
	}

	res := getAlertDetails()
	stringResponse := "Service ID : " + serviceIDs[0] +
		"\nStart Time : " + since +
		"\nEnd Time : " + until +
		"\n\nALERT NAME | OCCURANCE TIME | TTA | TTR \n"
	for alert := range res {
		stringResponse = stringResponse +
			res[alert].name + "|" +
			res[alert].occuranceTime + "|" +
			strconv.FormatFloat(res[alert].TTA, 'f', -1, 64) + "|" +
			strconv.FormatFloat(res[alert].TTR, 'f', -1, 64) + "\n"
	}
	w.Write([]byte(stringResponse))
}

func getAnalytics(w http.ResponseWriter, req *http.Request) {
	if t1, ok := req.URL.Query()["since"]; ok {
		since = t1[0]
		if t2, ok := req.URL.Query()["until"]; ok {
			until = t2[0]
		}
	}
	if ID, ok := req.URL.Query()["serviceID"]; ok {
		serviceIDs = ID
	}
	getAlertAnalytics(getAlertDetails())
}

func getSummary(w http.ResponseWriter, req *http.Request) {
	if t1, ok := req.URL.Query()["since"]; ok {
		since = t1[0]
		if t2, ok := req.URL.Query()["until"]; ok {
			until = t2[0]
		}
	}
	if ID, ok := req.URL.Query()["serviceID"]; ok {
		serviceIDs = ID
	}
	res := getAlertsSummary(getAlertAnalytics(getAlertDetails()))
	stringResponse := "Service ID : " + serviceIDs[0] +
		"\nStart Time : " + since +
		"\nEnd Time : " + until +
		"\n\nAlerts (Total) : " + strconv.Itoa(res.totalCount) +
		"\nAlerts (Off Hours) : " + strconv.Itoa(res.totalOffHourCount) +
		"\nAlerts (Weekends) : " + strconv.Itoa(res.totalWeekendCount) +
		"\nAlerts (Off Hours) : " + strconv.FormatFloat(res.offHourPercent, 'f', -1, 64) + "%" +
		"\nAlerts (Weeekends) : " + strconv.FormatFloat(res.weekendPercent, 'f', -1, 64) + "%" +
		"\nAlerts (Off Hours+Weekends) : " + strconv.FormatFloat(res.totalOffHourPercent, 'f', -1, 64) + "%" +
		"\nMean MTTR(s) : " + strconv.FormatFloat(res.MeanMTTR, 'f', -1, 64) +
		"\nMean MTTA(s) : " + strconv.FormatFloat(res.MeanMTTA, 'f', -1, 64)
	w.Write([]byte(stringResponse))
}

func getAlertDetails() []AlertDetails {
	incidents, err := getIncidents()
	if err != nil {
		panic(err)
	}
	var alertsDetails []AlertDetails
	for _, alert := range incidents {
		var alertDetail AlertDetails
		alertDetail.name = formatAlertName(alert.Title)
		alertDetail.occuranceTime = alert.CreatedAt
		alertDetail.TTR = getTimeDiff(alert.CreatedAt, alert.LastStatusChangeAt)

		logEntries, err := getIncidentLogEntry(alert.Id)
		if err == nil {
			TTA := math.MaxFloat64
			for _, logEntry := range logEntries {
				if strings.Contains(logEntry.Summary, "Acknowledged") {
					diff := getTimeDiff(alert.CreatedAt, logEntry.CreatedAt)
					if diff < TTA {
						TTA = diff
					}
				}
			}
			alertDetail.TTA = TTA
		}
		alertsDetails = append(alertsDetails, alertDetail)
	}
	return alertsDetails
}

func getIncidents() ([]pagerduty.Incident, error) {

	var opts pagerduty.ListIncidentsOptions
	opts.Since = since
	opts.Until = until
	opts.ServiceIDs = serviceIDs

	if alerts, err := client.ListIncidents(opts); err != nil {
		return nil, err
	} else {
		return alerts.Incidents, nil
	}
}

func getIncidentLogEntry(incidentID string) ([]pagerduty.LogEntry, error) {
	var opts pagerduty.ListIncidentLogEntriesOptions
	if alertLogEntry, err := client.ListIncidentLogEntries(incidentID, opts); err != nil {
		return nil, err
	} else {
		return alertLogEntry.LogEntries, nil
	}

}

func getTimeDiff(t1, t2 string) float64 {
	layout := "2006-01-02T15:04:05Z"
	ts1, err := time.Parse(layout, t1)
	if err == nil {
		ts2, err := time.Parse(layout, t2)
		if err == nil {
			return ts2.Sub(ts1).Seconds()
		}
	}
	return -1
}

func formatAlertName(name string) string {
	for _, s := range strings.Split(name, " ") {
		// Any formatting for the alert goes here.
	}
	return name
}

func getAlertAnalytics(alertsDetails []AlertDetails) map[string]AlertAnalytics {
	analytics := make(map[string]AlertAnalytics)
	for _, aDetail := range alertsDetails {
		if v, ok := analytics[aDetail.name]; ok {
			v.count += 1
			v.MTTA = (v.MTTA + aDetail.TTA) / 2
			v.MTTR = (v.MTTR + aDetail.TTR) / 2
			v.occuranceTimes = append(v.occuranceTimes, aDetail.occuranceTime)
			if isWeekend(aDetail.occuranceTime) {
				v.weekendCount += 1
			} else if isOffHour(aDetail.occuranceTime) {
				v.offHourCount += 1
			}
			analytics[aDetail.name] = v
		} else {
			v.name = aDetail.name
			v.count = 1
			v.MTTA = aDetail.TTA
			v.MTTR = aDetail.TTR
			v.occuranceTimes = append(v.occuranceTimes, aDetail.occuranceTime)
			if isWeekend(aDetail.occuranceTime) {
				v.weekendCount = 1
			} else if isOffHour(aDetail.occuranceTime) {
				v.offHourCount = 1
				v.weekendCount = 0
			} else {
				v.offHourCount = 0
				v.weekendCount = 0
			}
			analytics[aDetail.name] = v
		}
	}
	return analytics
}

func isWeekend(t string) bool {
	layout := "2006-01-02T15:04:05Z"
	ts, err := time.Parse(layout, t)
	if err == nil {
		if ts.Weekday() == time.Saturday || ts.Weekday() == time.Sunday {
			return true
		}
	}
	return false
}

func isOffHour(t string) bool {
	layout := "2006-01-02T15:04:05Z"
	ts, err := time.Parse(layout, t)
	if err == nil {
		if ts.Hour() < 9 || ts.Hour() > 17 {
			return true
		}
	}
	return false
}

func getAlertsSummary(alertAnalytics map[string]AlertAnalytics) AlertsSummary {
	var alertsSummary AlertsSummary
	for k := range alertAnalytics {
		alertsSummary.totalCount += alertAnalytics[k].count
		alertsSummary.totalOffHourCount += alertAnalytics[k].offHourCount
		alertsSummary.totalWeekendCount += alertAnalytics[k].weekendCount
		alertsSummary.MeanMTTA += alertAnalytics[k].MTTA
		alertsSummary.MeanMTTR += alertAnalytics[k].MTTR
	}
	alertsSummary.MeanMTTA = math.Round(alertsSummary.MeanMTTA/float64(alertsSummary.totalCount)*100) / 100
	alertsSummary.MeanMTTR = math.Round(alertsSummary.MeanMTTR/float64(alertsSummary.totalCount)*100) / 100
	alertsSummary.offHourPercent = math.Round((float64(alertsSummary.totalOffHourCount)/float64(alertsSummary.totalCount))*10000) / 100
	alertsSummary.weekendPercent = math.Round((float64(alertsSummary.totalWeekendCount)/float64(alertsSummary.totalCount))*10000) / 100
	alertsSummary.totalOffHourPercent = math.Round((float64(alertsSummary.totalWeekendCount+alertsSummary.totalOffHourCount)/float64(alertsSummary.totalCount))*10000) / 100
	return alertsSummary
}
