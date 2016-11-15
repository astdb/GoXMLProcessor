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

	// expecting two command-line arguments at invocation - API location for reading image data from and output directory for writing static site files
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

	// predefine the token literal values we're interested in for ease of comparison when processing the XML data body
	ID := "id"
	FILENAME := "filename"
	WORK := "work"
	MODEL := "model"
	MAKE := "make"
	URISMALL := "small"
	URIMEDIUM := "medium"
	URILARGE := "large"

	// read XML data body
	dec := xml.NewDecoder(apiDataResp.Body)

	var stack []string // we'll use a string slice as a stack datastructure to pop on/off start/end elements as we read through the XML data body's tokens
	var works []*Work  // collection of all works detected
	var makes []*Make  // collection of all makes detected

	var newWork *Work         // placeholder for the work cirrently being iterated through
	newModelDetected := false // flag indicating if a new model was detected in this work (if so to be added to the make of this work)
	// var newModel string
	newModel := "" // name of new model detected (if indicated by newModelDetected flag above)

	// URI data type flags
	thumbnailURI := false
	mediumURI := false
	largeURI := false

	// iterate through the full XML data body per detected token, until EOF
	for {
		// get next XML token
		token, err := dec.Token()

		// handle any errors
		if err == io.EOF {
			// reached end of data
			break
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading XML data body token: %v\n", err)
			os.Exit(1)
		}

		// fmt.Print(stack)

		// switch statement to take selective action based on the current token
		switch token := token.(type) {
		case xml.StartElement:
			// XML start element: push on stack and initialize new work object
			stack = append(stack, token.Name.Local)
			// fmt.Print(" (pushing " + token.Name.Local + ")")

			if len(stack) > 0 && stack[len(stack)-1] == WORK {
				// start of a new <work> in XML data: create a new Work instance and pop in to the list of all works
				// fmt.Printf("------------------------starting new work object------------------------")
				newWork = createWork()
				works = append(works, newWork)
			}

			if len(stack) > 0 && stack[len(stack)-1] == "url" {
				for _, val := range token.Attr {
					// fmt.Printf("Name: %v , Value: %s \n", key, val)
					if val.Value == URISMALL {
						thumbnailURI = true
					}

					if val.Value == URIMEDIUM {
						mediumURI = true
					}

					if val.Value == URILARGE {
						largeURI = true
					}
				}
			}

		case xml.EndElement:
			// XML end element: pop off stack
			elementPopped := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			// fmt.Println(" (popping " + token.Name.Local + ")")

			if elementPopped == WORK {
				// end of a <work> element (</work>)
				// fmt.Printf("------------------------ending work object------------------------")
				newWork = nil
				newModelDetected = false
				newModel = ""
			}
		case xml.CharData:
			// XML data - populate the current work object based on XML data token (e.g. ID, model, make etc.)

			//Work ID
			if len(stack) > 0 && stack[len(stack)-1] == ID {
				if len(works) > 0 {
					works[len(works)-1].ID, err = strconv.Atoi(strings.TrimSpace(string(token)))

					if err != nil {
						fmt.Fprintf(os.Stderr, "Error converting Work ID: %v\n", err)
						os.Exit(1)
					}
				}
			}

			// Work filename
			if len(stack) > 0 && stack[len(stack)-1] == FILENAME {
				// fmt.Printf("%s\n", token)
				if len(works) > 0 {
					works[len(works)-1].FileName = strings.TrimSpace(string(token))
				}
			}

			// Work camera make
			if len(stack) > 0 && stack[len(stack)-1] == MAKE {
				// make detected: retrieve make if already recorded, create if new
				thisToken := strings.TrimSpace(string(token))

				// if newWork != nil {
				var thisMake *Make

				if thisToken == "" {
					thisToken = "(Generic make)"
				}

				// check if this make is recorded in the global makes list
				makeFound := false
				for _, make := range makes {
					if make != nil && make.Name == thisToken {
						thisMake = make
						makeFound = true
						break
					}
				}

				if makeFound {
					// known make, populate work's make attribute with the make from the global list
					works[len(works)-1].WMake = thisMake
				} else {
					// new make: create and add to global makes list, and populate this work's make attribute with it
					thisMake = createMake(thisToken)
					makes = append(makes, thisMake)
					works[len(works)-1].WMake = thisMake
				}
				// } else {
				// fmt.Fprintf(os.Stderr, "Make detected with no active Work element - possibly malformed XML input")
				// os.Exit(1)
				// }

				// if len(works) > 0 {
				// 	works[len(works)-1].WMake = strings.TrimSpace(string(token))
				// }

				// camera model detected for this work?
				if newModelDetected == true {
					var thisModel *Model
					var thisWork *Work

					if len(works) > 0 {
						thisWork = works[len(works)-1]
					} else {
						fmt.Fprintf(os.Stderr, "No works recorded, but already processing a camera model - malformed XML?")
						os.Exit(1)
					}

					// if the model name is empty
					if newModel == "" {
						newModel = "(Generic model)"
					}

					// check if this model is recorded in this make's model list
					modelFound := false
					if thisWork.WMake != nil {
						for _, model := range thisWork.WMake.Models {
							if model != nil && model.Name == newModel {
								thisModel = model
								modelFound = true
							}
						}
					}

					if modelFound {
						thisWork.WModel = thisModel
					} else {
						thisModel = createModel(newModel, thisMake)

						if thisWork.WMake != nil {
							// thisWork.WMake.Models = append(newWork.WMake.Models, thisModel)
							thisWork.WMake.Models = append(thisWork.WMake.Models, thisModel)
							thisWork.WModel = thisModel
						}

						thisModel = nil
						newModel = ""
						// works[len(works)-1].WMake = thisMake
					}

					newModelDetected = false
				}
			}

			if len(stack) > 0 && stack[len(stack)-1] == MODEL {
				// model detected: add this model to the make.[]model of this work (if not already recorded)
				// fmt.Println("++++++++++Model detected++++++++++")
				thisToken := strings.TrimSpace(string(token))
				newModel = thisToken
				newModelDetected = true
			}

			if thumbnailURI == true {
				// fmt.Println(strings.TrimSpace(string(token)))
				if len(works) > 0 {
					works[len(works)-1].URISmall = strings.TrimSpace(string(token))
				}

				thumbnailURI = false
			}

			if mediumURI == true {
				if len(works) > 0 {
					works[len(works)-1].URIMedium = strings.TrimSpace(string(token))
				}

				mediumURI = false
			}

			if largeURI == true {
				if len(works) > 0 {
					works[len(works)-1].URILarge = strings.TrimSpace(string(token))
				}

				largeURI = false
			}
		}
	}

	// for _, m := range makes {
	// 	printMake(m)
	// }
	// fmt.Println()
	// for _, w := range works {
	// 	printWork(w)
	// }

	// ------- Generate index.html -------------------

	// check if the specified output directory exists - if not, create it
	fileInPlace, e := fileExists("./" + outputFolderLocation)

	if e != nil {
		fmt.Fprintf(os.Stderr, "Error checking output directory placement: %v\n", e)
		os.Exit(1)
	}

	if fileInPlace {
		fmt.Println("./" + outputFolderLocation + " in place.")
	} else {
		fmt.Println("./" + outputFolderLocation + " not in place - creating...")
		os.MkdirAll("./"+outputFolderLocation, 0755)
	}

	// open output file for writing
	outFileName := "./" + outputFolderLocation + "/index.html"
	f, err := os.Create(outFileName)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output disk file: %v\n", err)
		os.Exit(1)
	}

	defer f.Close()

	makesSelect := ""

	for _, mk := range makes {
		
	}

	indexTitle := "Welcome to Photos!"
	indexNavigation := `<select><option value="--">-- select a camera make</option>` + makesSelect + `</select>`
	indexContent := "Just some images"

	_, err = f.WriteString(`<!DOCTYPE html><html><head><title>Works Index</title><style type="text/css">nav { margin: 10px;	}</style></head><body><header><h1>` + indexTitle + `</h1><nav>` + indexNavigation + `</nav></header>` + indexContent + `</body></html>`)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output to disk file: %v\n", err)
		os.Exit(1)
	}

	f.Sync()
}

//----------------- Types -------------------------------

// type struct representing a photographic work
type Work struct {
	ID        int
	FileName  string
	WMake     *Make
	WModel    *Model
	URISmall  string
	URIMedium string
	URILarge  string
}

// type struct representing a camera make
type Make struct {
	ID      int
	Name    string
	Models  []*Model
	PageURL string
}

// type struct representing a camera model
type Model struct {
	ID      int
	MMake   *Make
	Name    string
	PageURL string
}

//----------------- Type Generator -------------------------------

func createMake(name string) *Make {
	var m Make
	m.Name = name

	return &m
}

func createModel(name string, make *Make) *Model {
	var m Model
	m.Name = name
	m.MMake = make

	return &m
}

func createWork() *Work {
	var w Work
	w.ID = -1
	w.FileName = ""
	w.WMake = nil
	w.WModel = nil

	return &w
}

//----------------- Type Methods -------------------------------

func (m *Make) GetName() string {
	return m.Name
}

//----------------- Utilities -------------------------------

func printMake(m *Make) {
	if m == nil {
		fmt.Println("<Invalid Make object>")
		return
	}

	// fmt.Printf("%s [%v]\n", m.Name, m.Models)
	fmt.Println(m.Name + "(" + strconv.Itoa(len(m.Models)) + ")")
	for _, model := range m.Models {
		fmt.Println("\t" + model.Name)
	}

	return
}

func printWork(w *Work) {
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

	wModelName := ""
	if w.WModel == nil {
		wModelName = "<Generic/undefined>"
	} else {
		wModelName = w.WModel.Name
	}

	/// fmt.Println("[" + strconv.Itoa(w.ID) + "| " + w.WModel + "]")
	fmt.Println("[" + strconv.Itoa(w.ID) + "| " + wMakeName + "| " + wModelName + "]")
	fmt.Println("\t Thumbnail: " + w.URISmall)
	fmt.Println("\t Medium: " + w.URIMedium)
	fmt.Println("\t Large: " + w.URILarge)
	return
}

// exists returns whether the given file or directory exists or not
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return true, err
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
