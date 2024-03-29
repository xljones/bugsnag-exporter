package daa

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/xander-jones/bugsnag-exporter/pkg/common"
)

type BugsnagDAAResponse struct {
	body        []byte              // the JSON body response
	rateLimit   BugsnagDAARateLimit // The number of API calls that can be made per minute
	retryAfter  int                 // The datetime stamp of when a retry should be made (rate limit refresh)
	link        BugsnagDAANextLink  // The link for the next set of data, if it exists
	xTotalCount int                 // The total number of errors or event objects in this search
	status      int                 // The response status code from the Bugsnag DAA
}
type BugsnagDAARateLimit struct {
	limit     int
	remaining int
}

type BugsnagDAANextLink struct {
	url string // The URL of the link in the `Link` header
	rel string // The direction of the link. Can be `next` or `prev`
}

func PrintHttpHeaders(res BugsnagDAAResponse) {
	common.PrintVerbose("X-Total-Count: %d", res.xTotalCount)
	common.PrintVerbose("Ratelimit:     %d", res.rateLimit.limit)
	common.PrintVerbose("Remaining:     %d", res.rateLimit.remaining)
	common.PrintVerbose("Link:          %s", queryUnescape(res.link.url))
	common.PrintVerbose("Retry-After:   %d", res.retryAfter)
}

func queryUnescape(encodedValue string) string {
	decodedValue, err := url.QueryUnescape(encodedValue)
	if err != nil {
		common.ExitWithError(1, err)
	}
	return decodedValue
}

func PrintHttpBody(res BugsnagDAAResponse) {
	//common.PrintVerbose(string(res.body))
}

var client = &http.Client{}
var PersonalAuthToken = "" // Personal Auth Token "Go-API"

/*
	Makes repeat calls to the Bugsnag Data Access API to fetch data, following
	`next` links until there are no more links to follow.

	This function is only used if the expected response is going to be an array,
	else use BugsnagGetObject()
*/
func BugsnagGetArray(url string) []map[string]interface{} {
	var thisCallNo int = 0
	var warningAccepted bool = false
	var res BugsnagDAAResponse
	var totalCallsRequired int = -1
	var elements []map[string]interface{}
	for {
		thisCallNo += 1
		res = MakeBugsnagDAAGet(url)
		if res.status == 429 {
			common.Print("API rate limit reached: Sleeping for %d seconds", res.retryAfter)
			time.Sleep(time.Duration(res.retryAfter) * time.Second)
		} else if res.status == 200 {
			var unmarshallBody []map[string]interface{}
			err := json.Unmarshal([]byte(res.body), &unmarshallBody)
			if err != nil {
				common.ExitWithErrorAndString(1, err, "JSON Unmarshalling failed")
			} else {
				elements = append(elements, unmarshallBody...)
				var callsRemainingToMake int = calcCallsRemaining(len(elements), res.xTotalCount, len(unmarshallBody))
				if totalCallsRequired == -1 {
					// only update this value on the first pass as subsequent calls might have < a full page size
					// (i.e. the last call which would disruput progress calculations)
					totalCallsRequired = callsRemainingToMake + 1
				}
				if callsRemainingToMake > 4 && !warningAccepted && !common.NoWarn {
					if getConfirmation("This request will make " + fmt.Sprint(callsRemainingToMake) + " further API calls. Do you want to continue?") {
						warningAccepted = true
					}
				}
				common.Print("Downloaded data page: %d of %d (%.1f%%)", thisCallNo, totalCallsRequired, float64((float64(thisCallNo)/float64(totalCallsRequired))*100))
			}
			if res.link.url != "" && res.link.rel == "next" {
				url = res.link.url
			} else {
				break
			}
		} else {
			common.Print("An unexpected error (HTTP/%d) occured when making a call to %s", res.status, url)
			common.Print("%s", res.body)
			// bugsnag.Notify()
			common.ExitWithString(1, "An error status was returned from the Bugsnag Data Access API")
		}
	}
	return elements
}

func calcCallsRemaining(elementsDownloaded int, totalElementsToDownload int, elementsOnThisPage int) int {
	common.PrintVerbose("Downloaded %d of %d elements (including this payload of %d elements)", elementsDownloaded, totalElementsToDownload, elementsOnThisPage)
	if elementsOnThisPage == 0 {
		return 0
	} else {
		var callsRemainingToMake int = int(math.Ceil(float64(totalElementsToDownload-elementsDownloaded) / float64(elementsOnThisPage)))
		return callsRemainingToMake
	}
}

/*
	Makes call to the Bugsnag Data Access API to fetch data, this won't return
	a `next` link as this should be a single object only, not an array

	To get an array of elements, use BugsnagGetArray()
*/
func BugsnagGetObject(url string) map[string]interface{} {
	var res BugsnagDAAResponse
	var element map[string]interface{}
	res = MakeBugsnagDAAGet(url)
	if res.status == 429 {
		common.Print("API rate limit reached: Sleeping for %d seconds", res.retryAfter)
		time.Sleep(time.Duration(res.retryAfter) * time.Second)
	} else if res.status == 200 {
		err := json.Unmarshal([]byte(res.body), &element)
		if err != nil {
			common.ExitWithErrorAndString(1, err, "JSON Unmarshalling failed")
		}
	} else {
		common.Print("An error (HTTP/%d) occured when making a call to %s", res.status, url)
		common.Print("%s", res.body)
		common.ExitWithString(1, "An error status was returned from the Bugsnag Data Access API")
	}
	return element
}

/*
	Makes a single call to the Bugsnag Data Access API based on the url provided. When
		the data rate limit is reached, backs off until it can continue making calls.
	Returns a `BugsnagDAAResponse` object which contains important headers, and the
		marshalled JSON body (in []byte form)
*/
func MakeBugsnagDAAGet(url string) BugsnagDAAResponse {
	var response BugsnagDAAResponse

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Add("Authorization", "token "+PersonalAuthToken)

	res, err := client.Do(req)
	if err != nil {
		common.ExitWithError(1000, err)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		common.ExitWithError(1000, err)
	}

	response.status = res.StatusCode
	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		common.PrintVerbose("[HTTP " + fmt.Sprint(res.StatusCode) + "] Success response received (" + res.Status + ")")
	} else if res.StatusCode == 429 {
		common.PrintVerbose("[HTTP 429] need to back-off")
	} else {
		common.PrintVerbose("[HTTP " + fmt.Sprint(res.StatusCode) + "] Error response received: " + res.Status)
	}

	response.rateLimit.limit = parseHeaderInt(res.Header["X-Ratelimit-Limit"])
	response.rateLimit.remaining = parseHeaderInt(res.Header["X-Ratelimit-Remaining"])
	response.xTotalCount = parseHeaderInt(res.Header["X-Total-Count"])
	response.retryAfter = parseHeaderInt(res.Header["Retry-After"])
	response.link = parseHeaderNext(res.Header["Link"])
	response.body = body

	// Prints important HTTP headers if running in verbose mode only.
	PrintHttpHeaders(response)

	// Prints the body of the response
	PrintHttpBody(response)

	return response
}

/*
	Parse a header string number into an int, throwing an error if an
	integer conversion does not succeed. Returns -1 if the header is empty
*/
func parseHeaderInt(headerValuesArray []string) int {
	canonicalHeader := canonicalHeader(headerValuesArray)
	if canonicalHeader == "" {
		return -1
	} else {
		headerValue, err := strconv.ParseInt(canonicalHeader, 10, 64)
		if err != nil {
			common.ExitWithErrorAndString(0, err, "An API response header returned an unexpected non-integer value")
			return -1 // unreachable, but compiler static analysis fails otherwise
		} else {
			return int(headerValue)
		}
	}
}

/*
	Returns the first element in the headers array. Any more than one header for the ones
	we care about would be unexpected, so just return the first.
	If the array is empty, just return an empty string
*/
func canonicalHeader(headerValuesArray []string) string {
	if len(headerValuesArray) > 0 {
		return headerValuesArray[0]
	} else {
		return ""
	}
}

/*
	Extract the next link and which direction it goes in relation to the current URL
*/
// TODO: make understand links like:
// {https://api.bugsnag.com/organizations/5919dd35488ed9001b53f3c3/projects?direction=desc&offset%!B(MISSING)null_sort_field%!D(MISSING)=false&offset%!B(MISSING)sort_field_offset%!D(MISSING)=60ab55a5c4fdc30014952c68&per_page=30&sort=created_at>; rel="next",<https://api.bugsnag.com/organizations/5919dd35488ed9001b53f3c3/projects?direction=desc&per_page=30&sort=created_at prev}
func parseHeaderNext(headerValuesArray []string) BugsnagDAANextLink {
	header := canonicalHeader(headerValuesArray)
	var rtn BugsnagDAANextLink
	if len(header) > 0 {
		r, _ := regexp.Compile("<(https://api.bugsnag.com/[^\"]*)>; rel=\"(next|prev)\"")
		matches := r.FindAllStringSubmatch(header, -1)
		rtn.url = matches[0][1]
		rtn.rel = matches[0][2]
	}
	return rtn
}
