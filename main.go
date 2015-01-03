package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"text/template"
	"strings"

	"github.com/GeertJohan/go.rice"
	"github.com/skratchdot/open-golang/open"
	"github.com/tomerdmnt/go-libGeoIP"
)

type GIJSON struct {
	Countries map[string]*Country `json:"countries"`
	Cities    []*City             `json:"cities"`
	Total     int                 `json:"total"`
}

type Country struct {
	Name string `json:"country"`
	Code string `json:"code"`
}

type City struct {
	Country   	string  `json:"country"`
	Name      	string  `json:"city"`
	Latitude  	float32 `json:"latitude"`
	Longitude 	float32 `json:"longitude"`
	Count     	int     `json:"count"`
	Ip        	string  `json:"ip"`
	PageRequest 	string	`json:"pagerequest"`
}

var gijson *GIJSON = &GIJSON{Countries: make(map[string]*Country), Cities: []*City{}}

func readStdin() {
	ipRe := regexp.MustCompile("((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)")
	pageRe := regexp.MustCompile(`\"(.*)\" ([0-9]+)`)
    	//match := re.FindStringSubmatch(`"GET /tory-burch-womens-rivington-pumps-for-197-8-sh/ HTTP/1.1" 200 8306 "-" "Mozilla/5.0 (compatible; Yahoo! Slurp; http://help.yahoo.com/help/us/ysearch/slurp)"`)
    	//fmt.Printf("4. %s\n", match[1])
	scanner := bufio.NewScanner(os.Stdin)

	DBBox := rice.MustFindBox("db")
	dbFile, err := DBBox.Open("GeoLiteCity.dat")
	if err != nil {
		log.Fatal(err)
	}
	geoip, err := libgeo.LoadFromReader(dbFile)
	if err != nil {
		log.Fatal(err)
	}

	for scanner.Scan() {
		if ip := ipRe.FindString(scanner.Text()); ip != "" && ip != "107.170.247.63" {
			if isBot(scanner.Text()) { break }
			if location := geoip.GetLocationByIP(ip); location != nil {
				pageRequest := pageRe.FindStringSubmatch(scanner.Text())
				processLocation(location,ip,pageRequest[1])
			}
		}
	}
}

func isBot(logEntry string) bool {

        // taken from http://www.asitis.com/16/

        botHandles := []string{
                "bot","crawler","spider","bingbot","Googlebot","ysearch","msnbot","Google-HTTP","metauri","Photon",
                "jetmon","FlipboardProxy","gzip","Twitterbot","TweetmemeBot","browserproxy","WordPress/4.1","http_request2",
                "crowsnest","alexa","firefly","froogle","ahrefsbot","pingdom","kraken","openhose","linkdex","grokkit",
                "cloudflare-alwaysonline","grouphigh","mj12bot","port-monitor","rqst","facebookexternalhit",
                "moreover","biggerbetter","inagist","incutio","blo.gs","feedbin","newspaper","typhoeus","recorded future",
                "linkfluence","netseer","package http","httplib2",
        }

        for each := range botHandles {
        	if strings.Contains(logEntry, each) {
        		return true
        	}
                //fmt.Printf("Divine value [%d] is [%s]\n", index, each)
        }
        return false
}

func processLocation(location *libgeo.Location, ip string, pageRequest string) {
	var found bool = false

	gijson.Countries[location.CountryName] = &Country{Name: location.CountryName}

	for _, c := range gijson.Cities {
		if c.Country == location.CountryName && c.Name == location.City {
			if c.PageRequest != pageRequest {
				c.PageRequest = c.PageRequest + "; <br />" + pageRequest
			}
			c.Count++
			found = true
			break
		}
	}
	if !found {
		city := &City{
			Country:   	location.CountryName,
			Name:      	location.City,
			Latitude:  	location.Latitude,
			Longitude: 	location.Longitude,
			Ip:	   	ip,
			PageRequest:	pageRequest,
			Count:     	1,
		}
		gijson.Cities = append(gijson.Cities, city)
	}
	gijson.Total++
}

func handleGIData(w http.ResponseWriter, r *http.Request) {
	json, err := json.Marshal(gijson)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(json)
}

func serveIndex(title string) func(http.ResponseWriter, *http.Request) {
	var html bytes.Buffer

	tmpl, err := rice.MustFindBox("templates").String("index.tmpl")
	if err != nil {
		log.Fatal(err)
	}
	t, err := template.New("index").Parse(tmpl)
	if err != nil {
		log.Fatal(err)
	}
	if err := t.Execute(&html, map[string]string{"Title": title}); err != nil {
		log.Fatal(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(html.Bytes())
	}
}

func main() {
	title := flag.String("title", "", "Optional Title")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: [geoipmap] [-title <title>] [-h]

    geoipmap reads logs from stdin
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	http.HandleFunc("/gidata", handleGIData)
	http.Handle("/resources/", http.FileServer(rice.MustFindBox("public").HTTPBox()))
	http.HandleFunc("/", serveIndex(*title))

	port, _ := strconv.Atoi(os.Getenv("PORT"))
	address := fmt.Sprintf("127.0.0.1:%d", port)

	go readStdin()

	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		addr := fmt.Sprintf("http://%s", l.Addr())
		fmt.Println(addr)
		open.Start(addr)
	}()
	log.Fatal(http.Serve(l, nil))
}
