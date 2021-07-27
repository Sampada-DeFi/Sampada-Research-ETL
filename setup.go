package main

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/storage"
	"golang.org/x/time/rate"
)

//RLHTTPClient Rate Limited HTTP Client
type RLHTTPClient struct {
	client      *http.Client
	Ratelimiter *rate.Limiter
}

func (c *RLHTTPClient) Do(req *http.Request) (*http.Response, error) {
	ctx := context.Background()
	err := c.Ratelimiter.Wait(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func NewClient(rl *rate.Limiter) *RLHTTPClient {
	c := &RLHTTPClient{
		client:      http.DefaultClient,
		Ratelimiter: rl,
	}
	return c
}

func GetRequestSEC(c *RLHTTPClient, userAgent string, url string) (io.ReadCloser, *gzip.Reader) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Accept-Encoding", "gzip,deflate")
	req.Header.Add("Host", "www.sec.gov")
	resp, _ := c.Do(req)
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	return resp.Body, gzr
}

func main() {
	ratelimiter := rate.NewLimiter(10, 10)
	c := NewClient(ratelimiter)
	// fmt.Println("Please enter in your Google Cloud project name: ")
	// var projectName string
	// fmt.Scanln(&projectName)
	// fmt.Println("Please enter in your Google Cloud Storage bucket name: ")
	// var bucketName string
	// fmt.Scanln(&bucketName)
	// fmt.Println("Please enter in your user agent in the form of (SampleCompanyName AdminContact@<sample company domain>.com) to use in your request header to the SEC")
	// var userAgent string
	// scanner := bufio.NewScanner(os.Stdin)
	// if scanner.Scan() {
	// 	userAgent = scanner.Text()
	// }

	//MAKE SURE TO DELETE BEFORE MERGING TO GITHUB!!!!!!!!!!!!!
	//MAKE SURE TO DELETE BEFORE MERGING TO GITHUB!!!!!!!!!!!!!
	//MAKE SURE TO DELETE BEFORE MERGING TO GITHUB!!!!!!!!!!!!!
	projectName := "sampada-research"
	bucketName := "sampada-research"
	userAgent := "Sampada-DeFi akoganti@sampada.io"
	//MAKE SURE TO DELETE BEFORE MERGING TO GITHUB!!!!!!!!!!!!!
	//MAKE SURE TO DELETE BEFORE MERGING TO GITHUB!!!!!!!!!!!!!
	//MAKE SURE TO DELETE BEFORE MERGING TO GITHUB!!!!!!!!!!!!!

	indexURL := "https://www.sec.gov/Archives/edgar/full-index/"
	resp, yearsJSON := GetRequestSEC(c, userAgent, indexURL+"index.json")
	var years EDGAR
	output, _ := io.ReadAll(yearsJSON)
	resp.Close()
	yearsJSON.Close()
	json.Unmarshal(output, &years)
	ctx := context.Background()
	bq, _ := bigquery.NewClient(ctx, projectName)
	ds := bq.Dataset("SEC")
	defer bq.Close()
	if err := ds.Create(ctx, &bigquery.DatasetMetadata{}); err != nil {
		fmt.Println(err)
	}
	balanceSheetTable := ds.Table("balance-sheet")
	balanceSheetSchema, _ := bigquery.InferSchema(BalanceSheetItem{})
	if err := balanceSheetTable.Create(ctx, &bigquery.TableMetadata{Schema: balanceSheetSchema}); err != nil {
		fmt.Println(err)
	}
	incomeStatementTable := ds.Table("income-statement")
	incomeStatementSchema, _ := bigquery.InferSchema(IncomeOrCashFlowStatementItem{})
	if err := incomeStatementTable.Create(ctx, &bigquery.TableMetadata{Schema: incomeStatementSchema}); err != nil {
		fmt.Println(err)
	}
	cashFlowStatementTable := ds.Table("cash-flow-statement")
	cashFlowStatementSchema, _ := bigquery.InferSchema(IncomeOrCashFlowStatementItem{})
	if err := cashFlowStatementTable.Create(ctx, &bigquery.TableMetadata{Schema: cashFlowStatementSchema}); err != nil {
		fmt.Println(err)
	}
	//Loop through each year
	for _, yearItem := range years.Directory.Item {
		//Testing for year 2020
		if yearItem.Type == "dir" && yearItem.Name == "2020" {
			year := yearItem.Name
			resp, qtrsJSON := GetRequestSEC(c, userAgent, indexURL+year+"/index.json")
			var qtrs EDGAR
			output, _ := io.ReadAll(qtrsJSON)
			resp.Close()
			qtrsJSON.Close()
			json.Unmarshal(output, &qtrs)
			//Loop through each quarter
			for _, qtrItem := range qtrs.Directory.Item {
				//Testing for qtr 1 in year 2020
				if qtrItem.Type == "dir" && qtrItem.Name == "QTR1" {
					qtr := qtrItem.Name
					//Get list of all xbrl filings
					resp, xbrlList := GetRequestSEC(c, userAgent, indexURL+year+"/"+qtr+"/xbrl.gz")
					body, _ := ioutil.ReadAll(xbrlList)
					resp.Close()
					xbrlList.Close()

					//Save xbrl filings list file to Google Cloud Storage
					ctx := context.Background()
					client, _ := storage.NewClient(ctx)
					defer client.Close()
					bkt := client.Bucket(bucketName)
					xbrlListObjectWriter := bkt.Object("SEC/" + year + "/" + qtr + "/xbrl.gz").NewWriter(ctx)
					if _, err := xbrlListObjectWriter.Write(body); err != nil {
						log.Fatal(err)
					}
					if err := xbrlListObjectWriter.Close(); err != nil {
						log.Fatal(err)
					}

					//Filter list for 10-Q and 10-K forms
					pattern := regexp.MustCompile(`---*`)
					loc := pattern.FindIndex(body)
					headerRow := [][]string{
						{"CIK", "CompanyName", "Form", "DateFiled", "FilingLoc"},
					}
					records := string(body)[loc[1]:]
					r := csv.NewReader(strings.NewReader(records))
					r.Comma = '|'
					csv, _ := r.ReadAll()
					df := append(headerRow, csv...)
					financialStatementsList := make([][]string, 0)
					for _, v := range df {
						if v[2] == "10-Q" || v[2] == "10-K" {
							financialStatementsList = append(financialStatementsList, v)
						}
					}

					j := 1
					//Loop through filings
					for i := range financialStatementsList {
						financialStatementsLoc := financialStatementsList[i][4]
						cik := financialStatementsList[i][0]

						if i < j {
							continue
						}

						if i > j {
							break
						}

						//Save whole filing in txt file to google cloud storage
						completeFilingURL := "https://www.sec.gov/Archives/" + financialStatementsLoc
						resp, filingTextFile := GetRequestSEC(c, userAgent, completeFilingURL)
						body, _ = ioutil.ReadAll(filingTextFile)
						resp.Close()
						xbrlList.Close()
						editedFilingLoc := strings.Replace(financialStatementsLoc, "edgar/data/", "", 1)
						pattern = regexp.MustCompile(`\d+\/`)
						loc = pattern.FindIndex([]byte(editedFilingLoc))
						filename := editedFilingLoc[loc[1]:]
						accessionNum := strings.Replace(strings.Replace(filename, "-", "", 2), ".txt", "", 1)
						filingTextFileObjectWriter := bkt.Object("SEC/" + year + "/" + qtr + "/" + accessionNum + "/" + filename).NewWriter(ctx)
						if _, err := filingTextFileObjectWriter.Write(body); err != nil {
							log.Fatal(err)
						}
						if err := filingTextFileObjectWriter.Close(); err != nil {
							log.Fatal(err)
						}

						//Find xbrl formatted balance sheet, income statement, and cash flow statement in Filing Summary
						filingDirectoryIndexURL := "https://www.sec.gov/Archives/" + strings.Replace(strings.Replace(financialStatementsLoc, "-", "", 2), ".txt", "", 1)
						fmt.Println(filingDirectoryIndexURL)
						filingSummaryURL := filingDirectoryIndexURL + "/FilingSummary.xml"
						resp, filingSummary := GetRequestSEC(c, userAgent, filingSummaryURL)
						filingSummaryFile, _ := io.ReadAll(filingSummary)
						var filingSummaryObject FilingSummary
						err := xml.Unmarshal(filingSummaryFile, &filingSummaryObject)
						if err != nil {
							log.Fatal(err)
						}
						resp.Close()
						filingSummary.Close()
						balanceSheetURL, incomeStatementURL, cashFlowStatementURL := ParseFilingSummary(filingSummaryObject, filingDirectoryIndexURL)

						//Parse Balance Sheet
						fmt.Println(balanceSheetURL, incomeStatementURL, cashFlowStatementURL)
						resp, balanceSheetHTML := GetRequestSEC(c, userAgent, balanceSheetURL)
						balanceSheet, _ := io.ReadAll(balanceSheetHTML)
						balanceSheetRows := ParseBalanceSheet(balanceSheet, year, qtr, cik)
						// fmt.Println(balanceSheetRows)
						resp.Close()
						balanceSheetHTML.Close()
						fmt.Println("Balance Sheet Parsed")
						// //Parse Income Statement
						// fmt.Println(incomeStatementURL)
						// resp, incomeStatementHTML := GetRequestSEC(c, userAgent, incomeStatementURL)
						// incomeStatement, _ := io.ReadAll(incomeStatementHTML)
						// incomeStatementRows := ParseIncomeOrCashFlowStatement(incomeStatement, year, qtr, cik)
						// resp.Close()
						// incomeStatementHTML.Close()
						// fmt.Println("Income Statement Parsed")
						// //Parse Cash Flow Statement
						// fmt.Println(cashFlowStatementURL)
						// resp, cashFlowStatementHTML := GetRequestSEC(c, userAgent, cashFlowStatementURL)
						// cashFlowStatement, _ := io.ReadAll(cashFlowStatementHTML)
						// cashFlowStatementRows := ParseIncomeOrCashFlowStatement(cashFlowStatement, year, qtr, cik)
						// resp.Close()
						// cashFlowStatementHTML.Close()
						// fmt.Println("Cash Flow Statement Parsed")
						//Upload financial data to BigQuery
						balanceSheetInserter := balanceSheetTable.Inserter()
						if err := balanceSheetInserter.Put(ctx, balanceSheetRows); err != nil {
							fmt.Println("Can't upload data balance sheet")
							log.Fatal(err)
						}
						// incomeStatementInserter := incomeStatementTable.Inserter()
						// if err := incomeStatementInserter.Put(ctx, incomeStatementRows); err != nil {
						// 	fmt.Println("Can't upload data income statement")
						// 	log.Fatal(err)
						// }
						// cashFlowStatementInserter := cashFlowStatementTable.Inserter()
						// if err := cashFlowStatementInserter.Put(ctx, cashFlowStatementRows); err != nil {
						// 	fmt.Println("Can't upload data cash flow statement")
						// 	log.Fatal(err)
						// }
					}
				}
			}
		}
	}
}
