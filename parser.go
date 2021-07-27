package main

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/anaskhan96/soup"
)

//Defining struct to save json object from index directories
type EDGAR struct {
	Directory struct {
		Item []struct {
			LastModified string `json:"last-modified"`
			Name         string `json:"name"`
			Type         string `json:"type"`
			Href         string `json:"href"`
			Size         string `json:"size"`
		} `json:"item"`
		Name      string `json:"name"`
		ParentDir string `json:"parent-dir"`
	} `json:"directory"`
}

//Defining struct to parse xml from filing summary
type FilingSummary struct {
	XMLName           xml.Name `xml:"FilingSummary"`
	Version           string   `xml:"Version"`
	ProcessingTime    string   `xml:"ProcessingTime"`
	ReportFormat      string   `xml:"ReportFormat"`
	ContextCount      string   `xml:"ContextCount"`
	ElementCount      string   `xml:"ElementCount"`
	EntityCount       string   `xml:"EntityCount"`
	FootnotesReported string   `xml:"FootnotesReported"`
	SegmentCount      string   `xml:"SegmentCount"`
	ScenarioCount     string   `xml:"ScenarioCount"`
	TuplesReported    string   `xml:"TuplesReported"`
	UnitCount         string   `xml:"UnitCount"`
	MyReports         struct {
		Report []struct {
			Instance           string `xml:"instance,attr"`
			IsDefault          string `xml:"IsDefault"`
			HasEmbeddedReports string `xml:"HasEmbeddedReports"`
			HtmlFileName       string `xml:"HtmlFileName"`
			LongName           string `xml:"LongName"`
			ReportType         string `xml:"ReportType"`
			Role               string `xml:"Role"`
			ShortName          string `xml:"ShortName"`
			MenuCategory       string `xml:"MenuCategory"`
			Position           string `xml:"Position"`
			ParentRole         string `xml:"ParentRole"`
		} `xml:"Report"`
	} `xml:"MyReports"`
	InputFiles struct {
		File []string `xml:"File"`
	} `xml:"InputFiles"`
	SupplementalFiles string `xml:"SupplementalFiles"`
	BaseTaxonomies    struct {
		BaseTaxonomy []string `xml:"BaseTaxonomy"`
	} `xml:"BaseTaxonomies"`
	HasPresentationLinkbase string `xml:"HasPresentationLinkbase"`
	HasCalculationLinkbase  string `xml:"HasCalculationLinkbase"`
}

type BalanceSheetItem struct {
	Year        string
	Quarter     string
	CIK         string
	Title       string
	Date        string
	Item        string
	Value       string
	Axis        string
	Abstract    string
	Tag         string
	Definition  string
	DataType    string
	BalanceType string
	PeriodType  string
}

type IncomeOrCashFlowStatementItem struct {
	Year        string
	Quarter     string
	CIK         string
	Title       string
	Date        string
	Item        string
	Value       string
	Duration    string
	Axis        string
	Abstract    string
	Tag         string
	Definition  string
	DataType    string
	BalanceType string
	PeriodType  string
}

func ParseFilingSummary(filingSummaryObject FilingSummary, filingDirectoryIndexURL string) (string, string, string) {
	balanceSheetNames := []string{"balance sheet", "statements of financial condition", "statements of condition"}
	incomeStatementNames := []string{"statements of income", "statements of operation", "statement of income", "statements of earnings", "statements of comprehensive loss", "statement of operations and comprehensive loss"}
	cashFlowStatementNames := []string{"statements of cash flow", "statement of cash flow"}
	balanceSheetFound := false
	incomeStatementFound := false
	cashFlowStatementFound := false
	var balanceSheetURL string
	var incomeStatementURL string
	var cashFlowStatementURL string
	for _, report := range filingSummaryObject.MyReports.Report {
		for _, name := range balanceSheetNames {
			if !balanceSheetFound && strings.Contains(strings.ToLower(report.LongName), name) && !strings.Contains(strings.ToLower(report.LongName), "parenthetical") {
				balanceSheetFound = true
				balanceSheetURL = filingDirectoryIndexURL + "/" + report.HtmlFileName
				break
			}
		}
		for _, name := range incomeStatementNames {
			if !incomeStatementFound && strings.Contains(strings.ToLower(report.LongName), name) {
				incomeStatementFound = true
				incomeStatementURL = filingDirectoryIndexURL + "/" + report.HtmlFileName
				break
			}
		}
		for _, name := range cashFlowStatementNames {
			if !cashFlowStatementFound && strings.Contains(strings.ToLower(report.LongName), name) {
				cashFlowStatementFound = true
				cashFlowStatementURL = filingDirectoryIndexURL + "/" + report.HtmlFileName
				break
			}
		}
		if balanceSheetFound && incomeStatementFound && cashFlowStatementFound {
			break
		}
	}
	return balanceSheetURL, incomeStatementURL, cashFlowStatementURL
}

func ParseBalanceSheet(balanceSheet []byte, year string, qtr string, cik string) []BalanceSheetItem {
	doc := soup.HTMLParse(string(balanceSheet))

	//checking to see if footnotes exist
	foundFootnotes := doc.Find("table", "class", "outerFootnotes").Error
	if foundFootnotes == nil {
		fmt.Println("Footnotes found")
	} else {
		fmt.Println("Footnotes not found: ", foundFootnotes)

	}

	columnHeadersFound := false
	var axes, abstracts, tags, definitions, dataTypes, balanceTypes, periodTypes, items, dates, footnotes []string
	var values [][]string
	axis, abstract, title := "", "", ""
	rows := doc.Find("table").FindAll("tr")
	for _, row := range rows {
		if !columnHeadersFound {
			for _, columnHeader := range row.FindAll("th") {
				if columnHeader.Attrs()["class"] == "tl" {
					title = columnHeader.FullText()
				}
				if columnHeader.Attrs()["class"] == "th" {
					dates = append(dates, columnHeader.FullText())
				}
			}
			columnHeadersFound = true
			values = make([][]string, len(dates))
			fmt.Println(title, dates, values)
			continue
		}
		index := 0
		for _, value := range row.FindAll("td") {
			switch class := value.Attrs()["class"]; class {
			case "pl ", "pl custom":
				xbrlTag := strings.Replace(strings.Replace(value.Find("a").Attrs()["onclick"], "top.Show.showAR( this, '", "", 1), "', window );", "", 1)
				if strings.Contains(xbrlTag, "Axis") {
					axis = xbrlTag
					break
				}
				if strings.Contains(xbrlTag, "Abstract") {
					abstract = xbrlTag
					break
				}
				// fmt.Println(value.FullText(), xbrlTag)
				items = append(items, value.FullText())
				tags = append(tags, xbrlTag)
				axes = append(axes, axis)
				abstracts = append(abstracts, abstract)
			case "nump", "num", "text":
				values[index] = append(values[index], value.FullText())
				// fmt.Println(value.FullText())
				index = index + 1
			}

			if value.Attrs()["class"] == "pl " || value.Attrs()["class"] == "pl custom" {
				xbrlTag := strings.Replace(strings.Replace(value.Find("a").Attrs()["onclick"], "top.Show.showAR( this, '", "", 1), "', window );", "", 1)
				if strings.Contains(xbrlTag, "Axis") {
					axis = xbrlTag
					break
				}
				if strings.Contains(xbrlTag, "Abstract") {
					abstract = xbrlTag
					break
				}
				// fmt.Println(value.FullText(), xbrlTag)
				items = append(items, value.FullText())
				tags = append(tags, xbrlTag)
				axes = append(axes, axis)
				abstracts = append(abstracts, abstract)
				continue
			}
			if value.Attrs()["class"] == "nump" || value.Attrs()["class"] == "num" || value.Attrs()["class"] == "text" {
				values[index] = append(values[index], value.FullText())
				// fmt.Println(value.FullText())
				index = index + 1
				continue
			}
			if value.Attrs()["class"] == "th" {
				footnotes = append(footnotes, value.FullText())
				continue
			}
			if value.Find("table").Error != nil {
				fmt.Println("Some empty row probably, couldn't find footnote just yet")
			}
		}
	}

	for _, tag := range tags {
		div := doc.Find("table", "id", tag).Find("tr").FindNextElementSibling().Find("div", "class", "body")
		definitions = append(definitions, div.Find("div").Find("p").Text())
		dataTypes = append(dataTypes, div.FindAll("div")[2].FindAll("tr")[2].FindAll("td")[1].Text())
		balanceTypes = append(balanceTypes, div.FindAll("div")[2].FindAll("tr")[3].FindAll("td")[1].Text())
		periodTypes = append(periodTypes, div.FindAll("div")[2].FindAll("tr")[4].FindAll("td")[1].Text())
	}

	fmt.Println(len(dates), len(items), len(values), len(values[1]), len(axes), len(abstracts), len(tags), len(definitions), len(dataTypes), len(balanceTypes), len(periodTypes))
	fmt.Println(footnotes)
	var balanceSheetRows []BalanceSheetItem
	for ii := range dates {
		for i := range items {
			balanceSheetRow := BalanceSheetItem{Year: year, Quarter: qtr, CIK: cik, Title: title, Date: dates[ii], Item: items[i], Value: values[ii][i], Axis: axes[i], Abstract: abstracts[i], Tag: tags[i], Definition: definitions[i], DataType: dataTypes[i], BalanceType: balanceTypes[i], PeriodType: periodTypes[i]}
			balanceSheetRows = append(balanceSheetRows, balanceSheetRow)
		}
	}
	return balanceSheetRows
}

func ParseIncomeOrCashFlowStatement(incomeOrCashFlowStatement []byte, year string, qtr string, cik string) []IncomeOrCashFlowStatementItem {
	doc := soup.HTMLParse(string(incomeOrCashFlowStatement))
	columnHeadersFound := false
	datesFound := false
	var columnHeaders, axes, abstracts, tags, definitions, dataTypes, balanceTypes, periodTypes, items []string
	var values [][]string
	var dates []string
	axis, abstract, title, duration := "", "", "", ""
	rows := doc.Find("table").FindAll("tr")
	for _, row := range rows {
		if !columnHeadersFound {
			for _, columnHeader := range row.FindAll("th") {
				columnHeaders = append(columnHeaders, columnHeader.FullText())
			}
			columnHeadersFound = true
			title = columnHeaders[0]
			duration = columnHeaders[1]
			continue
		}
		if !datesFound {
			for _, date := range row.FindAll("th") {
				dates = append(dates, date.FullText())
			}
			values = make([][]string, len(dates))
			datesFound = true
			fmt.Println(title, dates, values)
			continue
		}
		possibleXBRLTag := row.Find("td").Find("a")
		if possibleXBRLTag.Error != nil {
			fmt.Println(possibleXBRLTag.Error)
			continue
		}
		xbrlTag := strings.Replace(strings.Replace(possibleXBRLTag.Attrs()["onclick"], "top.Show.showAR( this, '", "", 1), "', window );", "", 1)
		fmt.Println(xbrlTag)
		if strings.Contains(xbrlTag, "Axis") {
			axis = xbrlTag
			continue
		}
		if strings.Contains(xbrlTag, "Abstract") {
			abstract = xbrlTag
			continue
		}
		index := 0
		for _, value := range row.FindAll("td") {
			if value.Attrs()["class"] == "pl" {
				items = append(items, value.FullText())
				fmt.Println(value.FullText())
				continue
			}
			if value.Attrs()["class"] == "th" {
				continue
			}
			if value.Attrs()["class"] == "nump" {
				values[index] = append(values[index], value.FullText())
				index = index + 1
				continue
			}
			if value.Attrs()["class"] == "text" {
				values[index] = append(values[index], value.FullText())
				index = index + 1
				continue
			}
			footnote := value.Find("table")
			if footnote.Error != nil {
				fmt.Println(footnote.Error)
			}
		}
		axes = append(axes, axis)
		abstracts = append(abstracts, abstract)
		tags = append(tags, xbrlTag)
	}

	for _, tag := range tags {
		div := doc.Find("table", "id", tag).Find("tr").FindNextElementSibling().Find("div", "class", "body")
		definitions = append(definitions, div.Find("div").Find("p").Text())
		dataTypes = append(dataTypes, div.FindAll("div")[2].FindAll("tr")[2].FindAll("td")[1].Text())
		balanceTypes = append(balanceTypes, div.FindAll("div")[2].FindAll("tr")[3].FindAll("td")[1].Text())
		periodTypes = append(periodTypes, div.FindAll("div")[2].FindAll("tr")[4].FindAll("td")[1].Text())
	}
	var incomeOrCashFlowStatementRows []IncomeOrCashFlowStatementItem
	for ii := range dates {
		for i := range items {
			incomeOrCashFlowStatementRow := IncomeOrCashFlowStatementItem{Year: year, Quarter: qtr, CIK: cik, Title: title, Date: dates[ii], Item: items[i], Value: values[ii][i], Duration: duration, Axis: axes[i], Abstract: abstracts[i], Tag: tags[i], Definition: definitions[i], DataType: dataTypes[i], BalanceType: balanceTypes[i], PeriodType: periodTypes[i]}
			incomeOrCashFlowStatementRows = append(incomeOrCashFlowStatementRows, incomeOrCashFlowStatementRow)
		}
	}
	return incomeOrCashFlowStatementRows
}