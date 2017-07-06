# gocrawl
Wordlist based HTTP and HTTPS crawler written in go

## Command Line
```
root@priv8box:~# gocrawl -h
Usage of gocrawl:
  -c value
    	A list of HTTP status considered as 'page found' ie: 200,302,304,401
  -concurrency int
    	Specify the concurrency connection at a time, a number between 10 and 900 (default 50)
  -cookie value
    	A list of Cookie used by application, format: name=value
  -depth int
    	Specify the maximum recursion depth (default 5)
  -proxy string
    	Specify HTTP proxy URL
  -scraper
    	Specify whenever to enable scraper engine
  -url string
    	Specify the url to crawl
  -wordlist string
    	Specify the wordlist used to crawl, if no specified the built-in one will be used
```

## Screenshot
![gocrawl](https://image.ibb.co/jKzMKa/Schermata_2017_07_05_alle_18_36_26.png)
