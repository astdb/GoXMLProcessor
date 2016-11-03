// reads works data from localhost/api/v1/works.xml and produces a set of static html files to navigate images.

package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func main() {
	fmt.Println("Image processor starting...")
	// fmt.Println(len(os.Args))

	if len(os.Args) <= 2 {
		fmt.Println("Error: please enter the image API URL and an output directory location as command-line arguments (e.g. >go run ImageProcessor http://localhost/test/api/v1/works.xml code/html/output)")
		return
	}

	imageAPILocation := os.Args[1]
	outputFolderLocation := os.Args[2]
	fmt.Printf("Accessing image API at %s\n", imageAPILocation)
	fmt.Printf("Output files at <%s>\n", outputFolderLocation)

	// get data from API location
	apiDataResp, err := http.Get(imageAPILocation)

	if err != nil {
		fmt.Fprintf(os.Stderr, "API fetch: %v\n", err)
		os.Exit(1)
	}

	apiDataRespBody, err := ioutil.ReadAll(apiDataResp.Body)
	apiDataResp.Body.Close()

	if err != nil {
		fmt.Fprintf(os.Stderr, "API fetch reading %s: %v\n", imageAPILocation, err)
		os.Exit(1)
	}

	// fmt.Printf("%s", apiDataRespBody)

	// unmarshall xml body
	var wl WorkList
	err = xml.Unmarshal(apiDataRespBody, &wl)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unmarshalling: %v\n", err)
		os.Exit(1)
	} else {
        fmt.Printf("XML data unmarshalled successfully.\n")
    }

	for _, thisWork := range wl.WorkList {
        // fmt.Printf("URLs: %v\n", thisWork.Url)
        fmt.Printf("URLs: %v\n", thisWork.EXIFData)
    }
}

// list of works to be read in from API
type WorkList struct {
	WorkList []Work `xml:"work"`
}

type Work struct {
	XMLName  xml.Name `xml:"work"`
	Url     string `xml:"small,attr"`
    // Where string `xml:"where,attr"`
	EXIFData []string `xml:"exif"`

	/* Name    string   `xml:"FullName"`
	   Phone   string
	   Email   []Email
	   Groups  []string `xml:"Group>Value"`
	   Address */
}

// ------------------- Internal Datatypes -------------------------------
// a photographic work
type WorkInternal struct {
	cameraMake   MakeInternal  // make of the camera which produced this photo
	cameraModel  ModelInternal // model of the camera which produced this photo
	thumbnailURL string        // thumbnail location for this photo
}

// a camera make
type MakeInternal struct {
	Name   string          // name of this camera make
	Models []ModelInternal // list of models of this make
}

// a camera model
type ModelInternal struct {
	modelName string       // name of this camera model
	modelMake MakeInternal // make of this camera model
}
