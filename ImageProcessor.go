// reads works data from localhost/api/v1/works.xml and produces a set of static html files to navigate images.

package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"html"
)

func main() {
	fmt.Println("Image processor starting...")

	// expecting two command-line arguments at invocation - API location for reading image data from and output directory for writing static site files
	if len(os.Args) <= 2 {
		fmt.Println("Error: please enter the image API URL and an output directory location as command-line arguments (e.g. >go run ImageProcessor http://localhost/test/api/v1/works.xml code/html/output)")
		return
	}

	// read in command line arguments: API URL and output directory
	imageAPILocation := os.Args[1]
	outputFolderLocation := os.Args[2]
	fmt.Printf("Accessing image API at %s\n", imageAPILocation)
	fmt.Printf("Output files will be written to <./%s>\n", outputFolderLocation)

	// get XML data from API location
	apiDataResp, err := http.Get(imageAPILocation)
	if err != nil {
		fmt.Fprintf(os.Stderr, "API fetch error: %v\n", err)
		os.Exit(1)
	}

	// read XML data body
	dec := xml.NewDecoder(apiDataResp.Body)

	// predefine the token literal values we're interested in for ease of comparison when processing the XML data body (e.g. <id>, <filename>, <work> etc)
	ID 			:= "id"
	FILENAME 	:= "filename"
	WORK 		:= "work"
	MODEL 		:= "model"
	MAKE 		:= "make"
	URISMALL 	:= "small"
	URIMEDIUM 	:= "medium"
	URILARGE 	:= "large"	

	var stack []string  // we'll use a string slice as a stack datastructure to pop on/off start/end elements as we read through the XML data body's tokens
	var works []*Work   // collection of all works detected
	var makes []*Make   // collection of all makes detected
	var worksSM []*Work // Works sans makes - if a work is found without a make speceifed, it'll go on this list and have a separate page generated for it to be diplayed

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

			if len(stack) > 0 && stack[len(stack)-1] == WORK {
				// start of a new <work> in XML data: create a new Work instance and pop in to the list of all works
				newWork = createWork()
				works = append(works, newWork)
			}

			if len(stack) > 0 && stack[len(stack)-1] == "url" {
				for _, val := range token.Attr {
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

			if elementPopped == WORK {
				// end of a <work> element (</work>)

				// add this work to its make and model's works lists
				thisWork := newWork
				thisMake := newWork.WMake
				thisModel := newWork.WModel

				if thisMake == nil {
					// record works without a make specified separately
					worksSM = append(worksSM, thisWork)
				}

				if thisMake != nil && thisModel != nil {
					thisMake.Works = append(thisMake.Works, thisWork)
					thisModel.Works = append(thisModel.Works, thisWork)
				}

				newWork = nil
				newModelDetected = false
				newModel = ""
				thisWork, thisMake, thisModel = nil, nil, nil
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
		fmt.Fprintf(os.Stderr, "Error creating index HTML file: %v\n", err)
		os.Exit(1)
	}

	defer f.Close()

	// dropdown navigation to all camera makes
	indexNavigation := `<select onchange="if (this.value) window.location.href=this.value"><option value="">-- select a camera make</option>`
	for _, mk := range makes {
		if mk != nil {
			indexNavigation = indexNavigation + `<option value="` + html.EscapeString(mk.PageURL) + `.html">` + html.EscapeString(mk.Name) + `</option>`
		}
	}

	if len(worksSM) > 0 {
		indexNavigation = indexNavigation + `<option value="nomake.html">(no make/generic)</option></select>`
	} else {
		indexNavigation = indexNavigation + `</select>`
	}

	// create thumbnails of first 10 works
	indexContent := ""

	imgCount := 0

	for _, wk := range works {
		indexContent = indexContent + `<img src=` + html.EscapeString(wk.URISmall) + `> `

		imgCount++
		if imgCount >= 10 {
			break
		}
	}

	_, err = f.WriteString(`<!DOCTYPE html><html><head><title>Welcome to Phoots!</title><style type="text/css">nav { margin: 10px;	}</style></head><body><header><h1>Welcome to Photos!</h1><nav>` + indexNavigation + `</nav></header>` + indexContent + `</body></html>`)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output to disk file: %v\n", err)
		os.Exit(1)
	}

	f.Sync()

	// ------------- Generate individual pages for each of the camera makes ------------------

	for _, mk := range makes {
		if mk != nil {
			outFileName := "./" + outputFolderLocation + "/" + mk.PageURL + ".html"
			f, err := os.Create(outFileName)

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating camera make page: %v\n", err)
				os.Exit(1)
			}

			defer f.Close()

			// dropdown navigation to all camera models of this make
			modelNavigation := `<select onchange="if (this.value) window.location.href=this.value"><option value="">-- select a camera model</option>`
			for _, md := range mk.Models {
				if md != nil {
					modelNavigation = modelNavigation + `<option value="` + html.EscapeString(md.PageURL) + `.html">` + html.EscapeString(md.Name) + `</option>`
				}
			}

			modelNavigation = modelNavigation + `</select>`

			// create thumbnails of first 10 works by this make
			makeContent := ""
			imgCount := 0
			// }

			for _, wk := range mk.Works {
				if wk != nil && wk.WMake != nil && wk.WMake.Name == mk.Name {
					makeContent = makeContent + `<img src=` + html.EscapeString(wk.URISmall) + `> `

					imgCount++
					if imgCount >= 10 {
						break
					}
				}
			}

			_, err = f.WriteString(`<!DOCTYPE html><html><head><title>All photos taken with a ` + html.EscapeString(mk.Name) + `</title><style type="text/css">nav { margin: 10px;	}</style></head><body><header><h1>All photos taken with a <i>` + mk.Name + `</i> camera</h1><nav><a href="index.html">back to homepage</a> | ` + modelNavigation + `</nav></header>` + makeContent + `</body></html>`)

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing output to make HTML file: %v\n", err)
				os.Exit(1)
			}

			f.Sync()
		}
	}

	// ------------- Generate separate page for works without a make ------------------
	if len(worksSM) > 0 {
		for _, wk := range worksSM {
			if wk != nil {
				outFileName := "./" + outputFolderLocation + "/nomake.html"
				f, err := os.Create(outFileName)

				if err != nil {
					fmt.Fprintf(os.Stderr, "Error creating generic works page: %v\n", err)
					os.Exit(1)
				}

				defer f.Close()

				// create thumbnails of first 10 works by this make
				genericContent := ""

				// imgCount := 0

				if wk != nil {
					genericContent = genericContent + `<img src=` + html.EscapeString(wk.URISmall) + `> `

					// imgCount++
					// if imgCount >= 10 {
					// 	break
					// }
				}

				_, err = f.WriteString(`<!DOCTYPE html><html><head><title>Generic Photographic Works</title><style type="text/css">nav { margin: 10px;	}</style></head><body><header><h1>Generic Photos</h1><nav><a href="index.html">back to homepage</a> </nav></header>` + genericContent + `</body></html>`)

				if err != nil {
					fmt.Fprintf(os.Stderr, "Error writing output to generic make works file: %v\n", err)
					os.Exit(1)
				}

				f.Sync()
			}
		}
	}

	// ------------- Generate individual pages for each of the camera models ------------------
	for _, mk := range makes {
		if mk != nil {
			for _, md := range mk.Models {
				if md != nil {
					outFileName := "./" + outputFolderLocation + "/" + md.PageURL + ".html"
					f, err := os.Create(outFileName)

					if err != nil {
						fmt.Fprintf(os.Stderr, "Error creating camera model page: %v\n", err)
						os.Exit(1)
					}

					defer f.Close()

					// create thumbnails of first 10 works by this make
					modelContent := ""

					imgCount := 0

					for _, wk := range md.Works {
						if wk != nil && wk.WMake != nil && wk.WMake.Name == mk.Name {
							modelContent = modelContent + `<img src=` + html.EscapeString(wk.URISmall) + `> `

							imgCount++
							if imgCount >= 10 {
								break
							}
						}
					}

					_, err = f.WriteString(`<!DOCTYPE html><html><head><title>All photos taken with a ` + html.EscapeString(md.Name) + `</title><style type="text/css">nav { margin: 10px;	}</style></head><body><header><h1>All photos taken with a <i>` + html.EscapeString(md.Name) + `</i> camera</h1><nav><a href="index.html">back to homepage</a> | <a href="` + html.EscapeString(mk.PageURL) + `.html">back to make</a></nav></header>` + modelContent + `</body></html>`)
				}
			}
		}
	}
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
	Works   []*Work
	PageURL string
}

// type struct representing a camera model
type Model struct {
	ID      int
	MMake   *Make
	Works   []*Work
	Name    string
	PageURL string
}

//----------------- Type Generator -------------------------------

func createMake(name string) *Make {
	var m Make
	m.Name = name

	// create the HTML filename for this make by stripping make name of all non-alphanumerics
	reg, err := regexp.Compile("[^A-Za-z0-9]+")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating make HTML filename: %v\n", err)
		os.Exit(1)
	}

	m.PageURL = reg.ReplaceAllString(name, "-")
	return &m
}

func createModel(name string, make *Make) *Model {
	var m Model
	m.Name = name
	m.MMake = make

	// create the HTML filename for this model by stripping model name of all non-alphanumerics
	reg, err := regexp.Compile("[^A-Za-z0-9]+")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating model HTML filename: %v\n", err)
		os.Exit(1)
	}

	m.PageURL = reg.ReplaceAllString(name, "-")

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

//----------------- Utility functions to print out works and makes for debug purposes -------------------------------

func printMake(m *Make) {
	if m == nil {
		fmt.Println("<Invalid Make object>")
		return
	}

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
