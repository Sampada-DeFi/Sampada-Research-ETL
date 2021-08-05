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
	Footnote    string
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
	Footnote    string
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

	//turn balance sheet into soup object for parsing
	doc := soup.HTMLParse(string(balanceSheet))

	//variables to store data and control flow
	columnHeadersFound := false
	var axes, abstracts, tags, definitions, dataTypes, balanceTypes, periodTypes, items, dates, footnotes []string
	var values [][]string
	axis, abstract, title := "", "", ""
	rows := doc.Find("table").FindAll("tr")

	//iterating over rows in balance sheet
RowLoopBS:
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
			//fmt.Println(title, dates, values)
			continue
		}
		index := 0
		//iterating over cells in row
		for _, value := range row.FindAll("td") {
			//each td tag has a tag that corresponds with some type of data that is consistent across all xbrl statements on the sec from what I've seen
			switch class := value.Attrs()["class"]; class {
			case "pl ", "pl custom":
				xbrlTag := strings.Replace(strings.Replace(value.Find("a").Attrs()["onclick"], "top.Show.showAR( this, '", "", 1), "', window );", "", 1)
				if strings.Contains(xbrlTag, "Axis") {
					axis = xbrlTag
					continue RowLoopBS
				}
				if strings.Contains(xbrlTag, "Abstract") {
					abstract = xbrlTag
					continue RowLoopBS
				}
				items = append(items, value.FullText())
				tags = append(tags, xbrlTag)
				axes = append(axes, axis)
				abstracts = append(abstracts, abstract)
			case "nump", "num", "text":
				values[index] = append(values[index], value.FullText())
				index = index + 1
			case "th":
				footnotes = append(footnotes, value.FullText())
			}
		}
	}
	// fmt.Println(values)
	// fmt.Println(tags)
	// fmt.Println(items)
	// fmt.Println(tags)
	// fmt.Println(axes)
	// fmt.Println(abstracts)

	for _, tag := range tags {
		div := doc.Find("table", "id", tag).Find("tr").FindNextElementSibling().Find("div", "class", "body")
		definitions = append(definitions, div.Find("div").Find("p").Text())
		dataTypes = append(dataTypes, div.FindAll("div")[2].FindAll("tr")[2].FindAll("td")[1].Text())
		balanceTypes = append(balanceTypes, div.FindAll("div")[2].FindAll("tr")[3].FindAll("td")[1].Text())
		periodTypes = append(periodTypes, div.FindAll("div")[2].FindAll("tr")[4].FindAll("td")[1].Text())
	}

	foundFootnotes := false
	//checking to see if footnotes exist
	footnotesTable := doc.Find("table", "class", "outerFootnotes").Error
	if footnotesTable == nil {
		fmt.Println("Footnotes found")
		foundFootnotes = true
	} else {
		fmt.Println("Footnotes not found: ", footnotesTable)
	}

	//need to get footnotes somehow
	if foundFootnotes {
		for index, footnoteNum := range footnotes {
			for _, footnote := range doc.Find("table", "class", "outerFootnotes").FindAll("tr") {
				if footnote.Find("td").FullText() == footnoteNum {
					// fmt.Println(footnote.Find("td").FullText())
					// fmt.Println(footnote.Find("td").FindNextElementSibling().FullText())
					footnotes[index] = footnote.Find("td").FindNextElementSibling().FullText()
				}
			}
		}
	} else {
		footnotes = make([]string, len(items))
	}
	fmt.Println(footnotes)
	// fmt.Println(len(dates), len(items), len(values), len(values[1]), len(axes), len(abstracts), len(tags), len(definitions), len(dataTypes), len(balanceTypes), len(periodTypes), len(footnotes))
	var balanceSheetRows []BalanceSheetItem
	for ii := range dates {
		for i := range items {
			balanceSheetRow := BalanceSheetItem{Year: year, Quarter: qtr, CIK: cik, Title: title, Date: dates[ii], Item: items[i], Value: values[ii][i], Axis: axes[i], Abstract: abstracts[i], Tag: tags[i], Definition: definitions[i], DataType: dataTypes[i], BalanceType: balanceTypes[i], PeriodType: periodTypes[i], Footnote: footnotes[i]}
			balanceSheetRows = append(balanceSheetRows, balanceSheetRow)
		}
	}
	return balanceSheetRows
}

func ParseIncomeOrCashFlowStatement(incomeOrCashFlowStatement []byte, year string, qtr string, cik string) []IncomeOrCashFlowStatementItem {

	//soup object to traverse html document
	doc := soup.HTMLParse(string(incomeOrCashFlowStatement))

	//variables and arrays to store data and control flow
	columnHeadersFound, multipleColumnFootnotesExist := false, false
	datesFound := false
	var axes, abstracts, tags, definitions, dataTypes, balanceTypes, periodTypes, items, footnotes []string
	var values [][]string
	var multipleColumnFootnotes [][]string
	var dates []string
	var details []soup.Root
	axis, abstract, title, duration := "", "", "", ""
	rows := doc.Find("table").FindAll("tr")

	//iterate over all financial statement rows
RowloopICFS:
	for _, row := range rows {

		//Getting title and time period of income/cash flow statement
		if !columnHeadersFound {
			for _, columnHeader := range row.FindAll("th") {
				if columnHeader.Attrs()["class"] == "tl" {
					title = columnHeader.FullText()
				}
				if columnHeader.Attrs()["class"] == "th" {
					duration = columnHeader.FullText()
				}
			}
			columnHeadersFound = true
			continue
		}
		if !datesFound {
			for _, date := range row.FindAll("th") {
				dates = append(dates, date.FullText())
			}
			values = make([][]string, len(dates))
			multipleColumnFootnotes = make([][]string, len(dates))
			datesFound = true
			fmt.Println(title, dates, values)
			continue
		}
		footnoteColumnIndex := 0
		index := 0
		//iterating over cells in row
		for _, value := range row.FindAll("td") {
			//each td tag has a tag that corresponds with some type of data that is consistent across all xbrl statements on the sec from what I've seen
			switch class := value.Attrs()["class"]; class {
			case "pl ", "pl custom":
				xbrlTag := strings.Replace(strings.Replace(value.Find("a").Attrs()["onclick"], "top.Show.showAR( this, '", "", 1), "', window );", "", 1)
				if strings.Contains(xbrlTag, "Axis") {
					axis = xbrlTag
					continue RowloopICFS
				}
				if strings.Contains(xbrlTag, "Abstract") {
					abstract = xbrlTag
					continue RowloopICFS
				}
				items = append(items, value.FullText())
				tags = append(tags, xbrlTag)
				axes = append(axes, axis)
				abstracts = append(abstracts, abstract)
			case "nump", "num", "text":
				values[index] = append(values[index], value.FullText())
				index = index + 1
			case "th":
				footnotes = append(footnotes, value.FullText())
			case "fn":
				multipleColumnFootnotesExist = true
				if multipleColumnFootnotesExist {
					fmt.Println("I'm going to kill myself")
				}
				multipleColumnFootnotes[footnoteColumnIndex] = append(multipleColumnFootnotes[footnoteColumnIndex], value.FullText())
				index = index + 1
			}
		}
	}

	fmt.Println(tags)

	for _, tag := range tags {
		fmt.Println(tag)
		div := doc.Find("table", "id", tag).Find("tr").FindNextElementSibling().Find("div", "class", "body")
		definitions = append(definitions, div.Find("div").Find("p").Text())
		fmt.Println()
		fmt.Println()
		for _, div := range div.FindAll("div") {
			if div.FindPrevElementSibling().FullText() == "+ Details" {
				details = div.FindAll("tr")
			}
		}
		dataTypes = append(dataTypes, details[2].FindAll("td")[1].Text())
		balanceTypes = append(balanceTypes, details[3].FindAll("td")[1].Text())
		periodTypes = append(periodTypes, details[4].FindAll("td")[1].Text())
	}

	foundFootnotes := false
	//checking to see if footnotes exist
	footnotesTable := doc.Find("table", "class", "outerFootnotes")
	if footnotesTable.Error == nil {
		fmt.Println("Footnotes found")
		fmt.Println(footnotesTable.FullText())
		foundFootnotes = true
	} else {
		fmt.Println("Footnotes not found: ", footnotesTable)
	}

	//need to get footnotes somehow
	if foundFootnotes {
		for index, footnoteNum := range footnotes {
			for _, footnote := range doc.Find("table", "class", "outerFootnotes").FindAll("tr") {
				footnotesDescription := ""
				for _, footnoteData := range footnote.FindAll("td") {
					if footnoteData.FullText() == footnoteNum {
						continue
					}
					footnotesDescription = footnotesDescription + footnoteData.FullText()
				}
				footnotes[index] = footnotesDescription
				// td := footnote.Find("td")
				// if td.FullText() == footnoteNum {
				// 	fmt.Println(td.FullText())
				// 	// fmt.Println(footnote.Find("td").FullText())
				// 	// fmt.Println(footnote.Find("td").FindNextElementSibling().FullText())
				// 	footnotes[index] = td.FindNextElementSibling().FullText()
				// }
			}
		}
	} else {
		footnotes = make([]string, len(items))
	}
	fmt.Println(len(dates), len(items), len(values), len(values[1]), len(axes), len(abstracts), len(tags), len(definitions), len(dataTypes), len(balanceTypes), len(periodTypes), len(footnotes))
	var incomeOrCashFlowStatementRows []IncomeOrCashFlowStatementItem
	if multipleColumnFootnotesExist {
		fmt.Println("gotta handle this shit")
		for ii := range dates {
			for i := range items {
				incomeOrCashFlowStatementRow := IncomeOrCashFlowStatementItem{Year: year, Quarter: qtr, CIK: cik, Title: title, Date: dates[ii], Item: items[i], Value: values[ii][i], Duration: duration, Axis: axes[i], Abstract: abstracts[i], Tag: tags[i], Definition: definitions[i], DataType: dataTypes[i], BalanceType: balanceTypes[i], PeriodType: periodTypes[i], Footnote: multipleColumnFootnotes[ii][i]}
				incomeOrCashFlowStatementRows = append(incomeOrCashFlowStatementRows, incomeOrCashFlowStatementRow)
			}
		}
	} else {
		for ii := range dates {
			for i := range items {
				incomeOrCashFlowStatementRow := IncomeOrCashFlowStatementItem{Year: year, Quarter: qtr, CIK: cik, Title: title, Date: dates[ii], Item: items[i], Value: values[ii][i], Duration: duration, Axis: axes[i], Abstract: abstracts[i], Tag: tags[i], Definition: definitions[i], DataType: dataTypes[i], BalanceType: balanceTypes[i], PeriodType: periodTypes[i], Footnote: footnotes[i]}
				incomeOrCashFlowStatementRows = append(incomeOrCashFlowStatementRows, incomeOrCashFlowStatementRow)
			}
		}
	}
	return incomeOrCashFlowStatementRows
}
