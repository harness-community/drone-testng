package plugin

import "encoding/xml"

// TestNGReport represents the structure of a TestNG XML report.
type TestNGReport struct {
	XMLName xml.Name `xml:"testng-results"`
	Suites  []Suite  `xml:"suite"`
}

// Suite represents a TestNG suite.
type Suite struct {
	Name     string  `xml:"name,attr"`
	Duration string  `xml:"duration-ms,attr"`
	Groups   []Group `xml:"groups>group"`
	Classes  []Class `xml:"test>class"`
}

// Group represents a TestNG group.
type Group struct {
	Name    string   `xml:"name,attr"`
	Methods []Method `xml:"method"`
}

// Method represents a method belonging to a group.
type Method struct {
	Name      string `xml:"name,attr"`
	Signature string `xml:"signature,attr"`
	ClassName string `xml:"class,attr"`
}

// Class represents a TestNG class.
type Class struct {
	Name  string `xml:"name,attr"`
	Tests []Test `xml:"test-method"`
}

// Test represents a TestNG test or configuration method.
type Test struct {
	Name        string `xml:"name,attr"`
	Status      string `xml:"status,attr"`
	DurationMS  string `xml:"duration-ms,attr"`
	IsConfig    bool   `xml:"is-config,attr"`
	Description string `xml:"description,attr"`
	Exception   string `xml:"exception>short-stacktrace"`
}

// Suite represents a TestNG suite.
type Results struct {
	Total      int
	Failures   int
	Skipped    int
	DurationMS float64
}
