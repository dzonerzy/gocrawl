package main

import(
	"flag"
	"fmt"
	"strings"
	"os"
	"io/ioutil"
	"time"
	"os/signal"
	"net/http"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"github.com/fatih/color"
)

type acceptedStatus []int

func (i *acceptedStatus) String() string {
	return fmt.Sprintf("%d", *i)
}

func (i *acceptedStatus) Set(value string) error {
	tmp, err := strconv.Atoi(value)
	if err != nil {
		*i = append(*i, -1)
	} else {
		*i = append(*i, tmp)
	}
	return nil
}

type CrawlerArguments struct {
	crawl_url string
	crawl_entries []string
	max_depth int
	concurrency int
	max_concurrency int
	valid_codes acceptedStatus
}

type CrawlerStatus struct {
	total_requests int
	request_per_second int
	total_ok int
	total_redirect int
	pages map[string]int
}

var status = CrawlerStatus{pages: make(map[string]int)}

func main() {

	var netTransport = &http.Transport{
		MaxIdleConnsPerHost: 50,
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	var netClient = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: time.Second * 5,
		Transport: netTransport,
	}

	var(
		url = flag.String("url","", "Specify the url to crawl")
		crawl_file = flag.String("wordlist", "", "Specify the wordlist used to crawl, if no specified the built-in one will be used")
		depth = flag.Int("depth", 5, "Specify the maximum recursion depth")
		concurrent = flag.Int("concurrency", 200, "Specify the concurrency connection at a time, a number between 10 and 900 default(200)")
	)
	var acceptedCodes acceptedStatus
	flag.Var(&acceptedCodes, "c", "A list of HTTP considered as 'Page Found' ie: 200,302,304,401")

	flag.Parse()

	if flag.NFlag() == 0 {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if len(acceptedCodes) == 0 {
		showError("You must declare at least one valid code")
		os.Exit(-1)
	}

	data, err := ioutil.ReadFile(*crawl_file)

	if err != nil{
		showError(fmt.Sprintf("Unable to open wordlist ( %v )", err))
		os.Exit(-1)
	}

	if *concurrent > 900 || *concurrent < 10 {
		showError("Error: Please set a concurrency value between 10 - 900")
		os.Exit(-1)
	}

	arguments := CrawlerArguments{crawl_url: *url, crawl_entries: strings.Split(string(data), "\n"),
		max_depth: *depth, concurrency:0, max_concurrency: *concurrent, valid_codes: acceptedCodes}

	if len(arguments.crawl_url) > 0 {
		go UpdateStats(&status, arguments)

		var wg sync.WaitGroup

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func(){
			for sig := range c {
				if sig.String() == "interrupt" {
					showInfo(fmt.Sprintf("Total Requests: %v , Request per sec: %v, HTTP OK: %v , HTTP Redirect: %v",
						status.total_requests, status.request_per_second, status.total_ok, status.total_redirect))
					os.Exit(0)
				}
			}
		}()

		showInfo(fmt.Sprintf("Starting crawling (%v)", arguments.crawl_url))

		if strings.HasPrefix(arguments.crawl_url, "http") || strings.HasPrefix(arguments.crawl_url, "https") {
			Crawl(&wg, arguments, 1, netClient)
		}else{
			arguments.crawl_url = "http://" + arguments.crawl_url
			Crawl(&wg, arguments, 1, netClient)
		}
	}else{
		showError("Error: you must insert a URL to crawl, use -h to check commandline options")
		os.Exit(-1)
	}

	showInfo(fmt.Sprintf("[Stats] Total Requests: %v , Request per sec: %v, HTTP OK: %v , HTTP Redirect: %v",
		status.total_requests, status.request_per_second, status.total_ok, status.total_redirect))
}

func showError(msg string) {
	HiRed := color.New(color.FgRed, color.Bold).SprintfFunc()
	fmt.Printf("[%s]: %s\n", HiRed("Error"), msg)
}

func showInfo(msg string) {
	HiRed := color.New(color.FgGreen, color.Bold).SprintfFunc()
	fmt.Printf("[%s]: %s\n", HiRed("INFO"), msg)
}

func showURL(url string, code string) {
	HiBlue := color.New(color.FgHiBlue, color.Bold).SprintfFunc()
	HiMagenta := color.New(color.FgHiMagenta, color.Bold).SprintfFunc()
	HiYellow := color.New(color.FgHiYellow, color.Bold).SprintfFunc()
	fmt.Printf("[%s]: %s %s %s\n", HiBlue("URL"), HiMagenta(url) , "=>" , HiYellow(code))
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}


func UpdateStats(status *CrawlerStatus, args CrawlerArguments){
	prev_total_requests := 0
	for {
		time.Sleep(time.Second * 1)
		if prev_total_requests > 0 {
			status.request_per_second = status.total_requests - prev_total_requests
			prev_total_requests = status.total_requests
		}else{
			prev_total_requests = status.total_requests
		}

		status.total_ok = 0
		status.total_redirect = 0

		for _ , code  := range status.pages{
			if contains(args.valid_codes, code) {
				status.total_ok++
			}else if code == 302 || code == 304{
				status.total_redirect++
			}
		}
	}
}

func ShouldGet(min, max int) bool {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max - min) + min > 0
}

func Request(wg *sync.WaitGroup,ch chan map[string]string , args CrawlerArguments, dir string, status *CrawlerStatus, client *http.Client) {
	url := args.crawl_url + "/" + dir
	var err error
	var response *http.Response
	var done bool = false
	for !done {
		if ShouldGet(-1, 1) {
			response, err = client.Get(url)
		} else {
			response, err = client.Head(url)
		}
		status.total_requests ++
		result := make(map[string]string)
		if err == nil {
			defer response.Body.Close()
			if !contains(args.valid_codes, response.StatusCode) {
				result[url] = ""
			} else {
				result[url] = strconv.Itoa(response.StatusCode)
				showURL(url, strconv.Itoa(response.StatusCode))
			}
			wg.Done()
			ch <- result
			done = true
		} else {
			time.Sleep(time.Millisecond * 100)
		}
	}
}

func Dispose(args CrawlerArguments, ch chan map[string]string) []string {
	var todo []string
	for i:=0;i<args.max_concurrency;i++ {
		select {
		case res := <-ch:
			for new_url,_ := range res {
				if len(res[new_url]) > 0 {
					todo = append(todo, new_url)
					val, _ := strconv.Atoi(res[new_url])
					status.pages[new_url] = val
				}
			}
		default:
		}
	}
	return todo
}

func Crawl(wg *sync.WaitGroup, args CrawlerArguments, depth int, client *http.Client) {
	var next_round [] string
	if depth <= args.max_depth {
		channel := make(chan map[string]string)
		for _, dir := range args.crawl_entries {
			if len(dir) > 0 {
				wg.Add(1)
				go Request(wg, channel, args, dir, &status, client)
				args.concurrency++
				if args.concurrency > args.max_concurrency {
					wg.Wait()
					next_round = append(next_round, Dispose(args, channel)...)
					args.concurrency = 0
				}
			}
		}

		if args.concurrency < args.max_concurrency {
			wg.Wait()
			next_round = append(next_round, Dispose(args, channel)...)
			args.concurrency = 0
		}

		for _, v := range next_round {
			args.crawl_url = v
			args.concurrency = 0
			Crawl(wg, args, depth+1, client)
		}
	}
}
