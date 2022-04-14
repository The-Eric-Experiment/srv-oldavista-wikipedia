package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

type Options map[string]string
type Params map[string]string

var defaultOptions Options = Options{
	"apiUrl": "http://en.wikipedia.org/w/api.php",
	"origin": "*",
}

func request(opt Options, pars Params) (map[string]interface{}, error) {
	base, err := url.Parse(opt["apiUrl"])
	if err != nil {
		return nil, err
	}

	requestOpt := make(Options)

	requestOpt["format"] = "json"
	requestOpt["action"] = "query"
	requestOpt["redirects"] = "1"
	requestOpt["origin"] = opt["origin"]

	// Query params
	params := url.Values{}
	for key, val := range requestOpt {
		params.Add(key, val)
	}

	for key, val := range pars {
		params.Add(key, val)
	}

	base.RawQuery = params.Encode()

	client := &http.Client{}

	req, err := http.NewRequest("GET", base.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("got error %s", err.Error())
	}
	req.Header.Set("User-Agent", "Old'aVista Search v1.0 (www.oldavista.com)")
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("got error %s", err.Error())
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("got error %s", err.Error())
	}
	responseMap := make(map[string]interface{})
	json.Unmarshal(body, &responseMap)
	return responseMap, nil
}

func handleRedirect(result map[string]interface{}, err error) (map[string]interface{}, error) {
	if err != nil {
		return result, err
	}

	query := result["query"].(map[string]interface{})
	var redirects []interface{} = make([]interface{}, 0)
	if query["redirects"] != nil {
		redirects = query["redirects"].([]interface{})
	}

	if len(redirects) == 1 {
		return request(defaultOptions, Params{
			"prop":        "info|pageprops|extracts",
			"inprop":      "url",
			"ppprop":      "disambiguation",
			"titles":      redirects[0].(map[string]interface{})["to"].(string),
			"explaintext": "1",
			"exintro":     "1",
		})
	}

	return result, err
}

func getFirstItem(result map[string]interface{}) (string, interface{}) {
	for k, v := range result {
		return k, v
	}

	return "", nil
}

type PageInfo struct {
	ID      float64 `json:"pageId"`
	Title   string  `json:"title"`
	Summary string  `json:"summary"`
	URL     string  `json:"url"`
}

func getValue[T any](input interface{}, key string) T {
	return input.(map[string]interface{})[key].(T)
}

func GetPage(t string) (*PageInfo, error) {
	result, err := handleRedirect(request(defaultOptions, Params{
		"prop":        "info|pageprops|extracts",
		"inprop":      "url",
		"ppprop":      "disambiguation",
		"titles":      t,
		"explaintext": "1",
		"exintro":     "1",
	}))

	if err != nil {
		return nil, err
	}

	pages := result["query"].(map[string]interface{})["pages"].(map[string]interface{})

	_, page := getFirstItem(pages)

	if page == nil {
		return nil, fmt.Errorf("no page found")
	}

	id := getValue[float64](page, "pageid")
	title := getValue[string](page, "title")
	url := getValue[string](page, "canonicalurl")
	summary := getValue[string](page, "extract")

	if err != nil {
		return nil, err
	}

	return &PageInfo{
		ID:      id,
		Title:   title,
		URL:     url,
		Summary: summary,
	}, nil
}

func getWikiPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	if q == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("q param is missing"))
		return
	}

	result, err := GetPage(q)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(result)
}

func main() {
	http.HandleFunc("/wikipage", getWikiPage)
	log.Fatal(http.ListenAndServe("0.0.0.0:8007", nil))
}
