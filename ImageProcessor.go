// reads works data from localhost/api/v1/works.xml and produces a set of static html files to navigate images.

package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	fmt.Printf("Output files will be written to <%s>\n", outputFolderLocation)

	// get XML data from API location
	apiDataResp, err := http.Get(imageAPILocation)
	if err != nil {
		fmt.Fprintf(os.Stderr, "API fetch error: %v\n", err)
		os.Exit(1)
	}

	// token literals predefined for comparison
	ID := "id"
	FILENAME := "filename"
	WORK := "work"
	MAKE := "make"
	MODEL := "model"

	// read XML data body
	dec := xml.NewDecoder(apiDataResp.Body)

	var stack []string // we'll use a stack of strings to pop in/off start/end elements as we read through the XML data body's tokens
	var works []*Work  // collection of all works detected
	var makes []*Make  // collection of all makes detected
	// var models []*Model	// collection of all models detected

	// per detected token, until EOF
	for {
		// get token
		token, err := dec.Token()

		// handle any errors
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading XML data body token: %v\n", err)
			os.Exit(1)
		}

		var newWork *Work

		// selective action based on token type
		switch token := token.(type) {
		case xml.StartElement:
			// XML start element: push on stack
			stack = append(stack, token.Name.Local)

			if len(stack) > 0 && stack[len(stack)-1] == WORK {
				// start of a new <work> in XML data: create a new Work instance and pop in to the list of all works
				newWork = createWork()
				works = append(works, newWork)
			}
		case xml.EndElement:
			// XML end element: pop off stack
			elementPopped := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			if elementPopped == WORK {
				// end of a <work> element (</work>)
				newWork = nil
			}
		case xml.CharData:
			// XML data
			if len(stack) > 0 && stack[len(stack)-1] == ID {
				if len(works) > 0 {
					works[len(works)-1].ID, err = strconv.Atoi(strings.TrimSpace(string(token)))

					if err != nil {
						fmt.Fprintf(os.Stderr, "Error converting Work ID: %v\n", err)
						os.Exit(1)
					}
				}
			}

			if len(stack) > 0 && stack[len(stack)-1] == FILENAME {
				// fmt.Printf("%s\n", token)
				if len(works) > 0 {
					works[len(works)-1].FileName = strings.TrimSpace(string(token))
				}
			}

			if len(stack) > 0 && stack[len(stack)-1] == MAKE {
				// retrieve make if already recorded, create if new
				thisToken := strings.TrimSpace(string(token))
				var thisMake *Make

				if thisToken == "" {
					thisToken = "(Generic make)"
				}

				makeFound := false
				for _, make := range makes {
					if make != nil && make.Name == thisToken {
						thisMake = make
						makeFound = true
						break
					}
				}

				if makeFound {
					works[len(works)-1].WMake = thisMake
				} else {
					thisMake = createMake(thisToken)
					makes = append(makes, thisMake)
					works[len(works)-1].WMake = thisMake
				}

				// if len(works) > 0 {
				// 	works[len(works)-1].WMake = strings.TrimSpace(string(token))
				// }
			}

			if len(stack) > 0 && stack[len(stack)-1] == MODEL {
				if len(works) > 0 {
					works[len(works)-1].WModel = strings.TrimSpace(string(token))
				}
			}
		}
	}

	for _, m := range makes {
		printMakes(m)
	}
	fmt.Println()
	for _, w := range works {
		printWorks(w)
	}
}

//----------------- Types -------------------------------

// type struct representing a photographic work
type Work struct {
	ID       int
	FileName string
	WMake    *Make
	WModel   string
}

// type struct representing a camera make
type Make struct {
	ID     int
	Name   string
	Models []Model
	Page   string
}

// type struct representing a camera model
type Model struct {
	ID   int
	Make Make
	Name string
	Page string
}

//----------------- Type Generator -------------------------------

func createMake(name string) *Make {
	var m Make
	m.Name = name

	return &m
}

func createModel(name string, make Make) *Model {
	var m Model
	m.Name = name
	m.Make = make

	return &m
}

func createWork() *Work {
	var w Work
	w.ID = -1
	w.FileName = ""
	w.WMake = nil
	w.WModel = ""

	return &w
}

//----------------- Type Methods -------------------------------

func (m *Make) GetName() string {
	return m.Name
}

//----------------- Utilities -------------------------------

func printMakes(m *Make) {
	if m == nil {
		fmt.Println("<Invalid Make object>")
		return
	}

	fmt.Println(m.Name)
	return
}

func printWorks(w *Work) {
	if w == nil {
		fmt.Println("<Invalid work object>")
		return
	}

	wMakeName := ""
	if w.WMake == nil {
		wMakeName = "<Generic/undefined>"
	} else {
		wMakeName = w.WMake.Name
	}

	/// fmt.Println("[" + strconv.Itoa(w.ID) + "| " + w.WModel + "]")
	fmt.Println("[" + strconv.Itoa(w.ID) + "| " + wMakeName + "| " + w.WModel + "]")
	return
}

// containsAll reports whether x contains the elements of y, in order.
func containsAll(x, y []string) bool {
	for len(y) <= len(x) {
		if len(y) == 0 {
			return true
		}
		if x[0] == y[0] {
			y = y[1:]
		}
		x = x[1:]
	}
	return false
}

//!-
