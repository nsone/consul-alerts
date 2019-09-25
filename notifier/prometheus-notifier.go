package notifier

import (
	"fmt"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	log "github.com/AcalephStorage/consul-alerts/Godeps/_workspace/src/github.com/Sirupsen/logrus"
)

type PrometheusNotifier struct {
	Enabled     bool
	ClusterName string   `json:"cluster-name"`
	BaseURLs    []string `json:"base-urls"`
	Endpoint    string   `json:"endpoint"`
}

// NotifierName provides name for notifier selection
func (notifier *PrometheusNotifier) NotifierName() string {
	return "prometheus"
}

func (notifier *PrometheusNotifier) Copy() Notifier {
	n := *notifier
	return &n
}

//Notify sends messages to the endpoint notifier
func (notifier *PrometheusNotifier) Notify(messages Messages) bool {
	var values []map[string]map[string]string

	// TODO make it a template so that user can control what to send
	for _, m := range messages {
		values = append(values, map[string]map[string]string{
			"labels": {
				"alertName": fmt.Sprintf("%s/%s/%s", m.Check, m.Service, m.Node),
				"host": m.Node,
				"service": m.Service,
				"severity": m.Status,
				"output": m.Output,
				"notes": m.Notes,
			},
		})
	}

	requestBody, err := json.Marshal(values)
	if err != nil {
		log.Println("Unable to encode POST data")
		return false
	}

	c := make(chan bool)
	defer close(c)
	for _, bu := range notifier.BaseURLs {
		endpoint := fmt.Sprintf("%s%s", bu, notifier.Endpoint)

		// Channel senders. Logging the result where needed, and sending status back
		go func() {
			if res, err := http.Post(endpoint, "application/json", bytes.NewBuffer(requestBody)); err != nil {
				log.Printf(fmt.Sprintf("Unable to send data to Prometheus server (%s): %s", endpoint, err))
				c <- false
			} else {
				defer res.Body.Close()
				statusCode := res.StatusCode

				if statusCode != 200 {
					body, _ := ioutil.ReadAll(res.Body)
					log.Printf(fmt.Sprintf("Unable to notify Prometheus server (%s): %s", endpoint, string(body)))
					c <- false
				} else {
					log.Printf(fmt.Sprintf("Notification sent to Prometheus server (%s).", endpoint))
					c <- true
				}
			}
		}()
	}

	// Channel receiver. Making sure to return the final result in bool
	result := true
	for i := 0; i < len(notifier.BaseURLs); i++ {
		select {
		case r := <- c:
			if (! r) {
				result = false
			}
		}
	}

	return result
}
