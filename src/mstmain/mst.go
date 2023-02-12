/*
This is a web application.  The backend server is written in Go and uses the
html/package to create the html used by the web browser, which points to localhost:8080/primmst.
Prim minimum spanning tree (MST) finds the minimum path length given the vertices.
Plot the MST showing the vertices and edges connecting the vertices in the web browser.
The user enters the following data in an html form:  #vertices, starting vertex, x-y bounds.
A random number of vertices is chosen for the initial connection with a random start vertex.
The user can select a different starting vertex.  The total distance of the MST is displayed.
*/

package main

import (
	"bufio"
	"container/heap"
	"fmt"
	"log"
	"math"
	"math/cmplx"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
)

const (
	addr           = "127.0.0.1:8080"         // http server listen address
	fileConnect    = "templates/primmst.html" // html for Prim MST
	patternConnect = "/primmst"               // http handler for Prim MST
	rows           = 300                      // #rows in grid
	columns        = 300                      // #columns in grid
	xlabels        = 11                       // # labels on x axis
	ylabels        = 11                       // # labels on y axis
	dataDir        = "data/"                  // directory for the data files
	fileVerts      = "vertices.csv"           // bounds and complex locations of vertices
)

// Edges are the vertices of the edge endpoints
type Edge struct {
	v int // one vertix
	w int // the other vertix
}

// Items are stored in the Priority Queue
type Item struct {
	Edge             // embedded field accessed with v,w
	index    int     // The index is used by Priority Queue update and is maintained by the heap.Interface
	distance float64 // Edge distance between vertices
}

// Priority Queue is a map of indexes and queue items and implements the heap.Interface
// A map is used instead of a slice so that it can be easily determined if an edge is in the queue
type PriorityQueue map[int]*Item

// Minimum spanning tree holds the edge vertices
type MST []*Edge

// Type to contain all the HTML template actions
type PlotT struct {
	Grid   []string // plotting grid
	Status string   // status of the plot
	Xlabel []string // x-axis labels
	Ylabel []string // y-axis labels
}

// Type to hold the minimum and maximum data values
type Endpoints struct {
	xmin float64
	xmax float64
	ymin float64
	ymax float64
}

// PrimMST type used by the http handler methods to create the MST
type PrimMST struct {
	graph    [][]float64  // matrix of vertices and their distance from each other
	location []complex128 // complex point(x,y) coordinates
	mst      MST
	Endpoints
}

// global variables for parse and execution of the html template and MST construction
var (
	tmplForm *template.Template
	primmst  *PrimMST
)

// init parses the html template fileS
func init() {
	tmplForm = template.Must(template.ParseFiles(fileConnect))
}

// generateVertices creates random vertices in the complex plane
func (p *PrimMST) generateVertices(r *http.Request) error {

	// new start vertex using saved vertices in csv file
	newstartvert := r.PostFormValue("newstartvert")
	if len(newstartvert) > 0 {
		f, err := os.Open(fileVerts)
		if err != nil {
			fmt.Printf("Open file %s error: %v\n", fileVerts, err)
		}
		defer f.Close()
		input := bufio.NewScanner(f)
		input.Scan()
		line := input.Text()
		// Each line has comma-separated values
		values := strings.Split(line, ",")
		var xmin, ymin, xmax, ymax float64
		if xmin, err = strconv.ParseFloat(values[0], 64); err != nil {
			fmt.Printf("String %s conversion to float error: %v\n", values[0], err)
			return err
		}

		if ymin, err = strconv.ParseFloat(values[1], 64); err != nil {
			fmt.Printf("String %s conversion to float error: %v\n", values[1], err)
			return err
		}
		if xmax, err = strconv.ParseFloat(values[0], 64); err != nil {
			fmt.Printf("String %s conversion to float error: %v\n", values[2], err)
			return err
		}

		if ymax, err = strconv.ParseFloat(values[1], 64); err != nil {
			fmt.Printf("String %s conversion to float error: %v\n", values[3], err)
			return err
		}
		p.Endpoints = Endpoints{xmin: xmin, ymin: ymin, xmax: xmax, ymax: ymax}

		p.location = make([]complex128, 0)
		for input.Scan() {
			line := input.Text()
			// Each line has comma-separated values
			values := strings.Split(line, ",")
			var x, y float64
			if x, err = strconv.ParseFloat(values[0], 64); err != nil {
				fmt.Printf("String %s conversion to float error: %v\n", values[0], err)
				continue
			}
			if y, err = strconv.ParseFloat(values[1], 64); err != nil {
				fmt.Printf("String %s conversion to float error: %v\n", values[1], err)
				continue
			}
			p.location = append(p.location, complex(x, y))
		}
		// Change starting vertex at 0 index
		swap := rand.Intn(len(p.location))
		p.location[0], p.location[swap] = p.location[swap], p.location[0]

		return nil
	}
	// Generate V vertices and locations randomly, get from HTML form
	// or read in from a previous graph when using a new start vertex.
	// Insert vertex complex coordinates into locations
	str := r.FormValue("xmin")
	xmin, err := strconv.ParseFloat(r.FormValue("xmin"), 64)
	if err != nil {
		fmt.Printf("String %s conversion to float error: %v\n", str, err)
		return err
	}

	str = r.FormValue("ymin")
	ymin, err := strconv.ParseFloat(r.FormValue("ymin"), 64)
	if err != nil {
		fmt.Printf("String %s conversion to float error: %v\n", str, err)
		return err
	}

	str = r.FormValue("xmax")
	xmax, err := strconv.ParseFloat(r.FormValue("xmax"), 64)
	if err != nil {
		fmt.Printf("String %s conversion to float error: %v\n", str, err)
		return err
	}

	str = r.FormValue("ymax")
	ymax, err := strconv.ParseFloat(r.FormValue("ymax"), 64)
	if err != nil {
		fmt.Printf("String %s conversion to float error: %v\n", str, err)
		return err
	}

	p.Endpoints = Endpoints{xmin: xmin, ymin: ymin, xmax: xmax, ymax: ymax}

	vertices := r.FormValue("vertices")
	verts, err := strconv.Atoi(vertices)
	if err != nil {
		fmt.Printf("String %s conversion to int error: %v\n", vertices, err)
		return err
	}

	delx := xmax - xmin
	dely := ymax - ymin
	// Generate vertices
	p.location = make([]complex128, verts)
	for i := 0; i < verts; i++ {
		x := xmin + delx*rand.Float64()
		y := ymin + dely*rand.Float64()
		p.location[i] = complex(x, y)
	}

	// Save the endpoints and vertex locations to a csv file
	f, err := os.Create(fileVerts)
	if err != nil {
		fmt.Printf("Create file %s error: %v\n", fileVerts, err)
		return err
	}
	defer f.Close()
	// Save the endpoints
	fmt.Fprintf(f, "%f,%f,%f,%f\n", p.xmin, p.ymin, p.xmax, p.ymax)
	// Save the vertex locations as x,y
	for _, z := range p.location {
		fmt.Fprintf(f, "%f,%f\n", real(z), imag(z))
	}

	return nil
}

// findDistances find distances between vertices and insert into graph
func (p *PrimMST) findDistances() error {

	verts := len(p.location)
	// Store distances between vertices for Euclidean graph
	p.graph = make([][]float64, verts)
	for i := 0; i < verts; i++ {
		p.graph[i] = make([]float64, verts)
	}

	for i := 0; i < verts; i++ {
		for j := i + 1; j < verts; j++ {
			distance := cmplx.Abs(p.location[i] - p.location[j])
			p.graph[i][j] = distance
			p.graph[j][i] = distance
		}
	}
	for i := 0; i < verts; i++ {
		p.graph[i][i] = math.MaxFloat64
	}

	return nil
}

// A PriorityQueue implements heap.Interface and holds Items
func (pq *PriorityQueue) Len() int {
	return len(*pq)
}

func (pq *PriorityQueue) Less(i, j int) bool {
	return (*pq)[i].distance < (*pq)[j].distance
}

func (pq *PriorityQueue) Swap(i, j int) {
	(*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i]
	(*pq)[i].index = i
	(*pq)[j].index = j
}

// Push inserts an Item in the queue
func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	(*pq)[n] = item
}

// Pop removes an Item from the queue and returns it
func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	delete(*pq, n-1)
	return item
}

// update modifies the distance and value of an Item in the queue
func (pq *PriorityQueue) update(item *Item, distance float64) {
	item.distance = distance
	heap.Fix(pq, item.index)
}

// findMST finds the minimum spanning tree using Prim's algorithm
func (p PrimMST) findMST() error {
	vertices := len(p.location)
	edgeTo := make([]Edge, vertices)
	marked := make([]bool, vertices)
	distTo := make([]float64, vertices)
	for i := range distTo {
		distTo[i] = math.MaxFloat64
	}
	// Create a priority queue, put the items in it, and establish
	// the priority queue (heap) invariants.
	pq := make(PriorityQueue)

	visit := func(v int) {
		marked[v] = true
		// find shortest distance from vertex v to w
		for w, dist := range p.graph[v] {
			// Check if already in the MST
			if marked[w] {
				continue
			}
			if dist < distTo[w] {
				// Edge to w is new best connection from MST to w
				edgeTo[w] = Edge{v: v, w: w}
				distTo[w] = dist
				// Check if already in the queue and update
				item, ok := pq[w]
				// update
				if ok {
					pq.update(item, dist)
				} else { // insert
					item = &Item{Edge: Edge{v: v, w: w}, distance: dist}
					heap.Push(&pq, item)
				}
			}
		}
	}

	// Starting index is 0, distance is 0, put it in the queue
	distTo[0] = 0.0
	pq[0] = &Item{index: 0, distance: 0.0, Edge: Edge{v: 0, w: 0}}
	heap.Init(&pq)

	// Loop until the queue is empty
	for len(pq) > 0 {
		item := heap.Pop(&pq).(*Item)
		visit(item.v)
	}

	return nil
}

// plotMST draws the MST onto the canvas (grid)
func (p *PrimMST) plotMST(w http.ResponseWriter, status []string) error {

	// Apply the parsed HTML template to plot object
	// Construct x-axis labels, y-axis labels, status message

	var (
		plot   PlotT
		xscale float64
		yscale float64
	)
	plot.Grid = make([]string, rows*columns)
	plot.Xlabel = make([]string, xlabels)
	plot.Ylabel = make([]string, ylabels)

	// Calculate scale factors for x and y
	xscale = (columns - 1) / (p.xmax - p.xmin)
	yscale = (rows - 1) / (p.ymax - p.ymin)

	// Insert the mst vertices and edges in the canvas
	// loop over the MST vertices
	for _, e := range p.mst {
		x := real(p.location[e.v])
		y := imag(p.location[e.w])
		row := int((p.ymax-y)*yscale + .5)
		col := int((x-p.xmin)*xscale + .5)
		plot.Grid[row*columns+col] = "vertex"
	}

	// color the vertices black
	// color the edges connecting the vertices gray
	// create the line y = mx + b for each edge
	// translate complex coordinates to row/col on the canvas
	// translate row/col to slice data object []string
	// CSS selectors for background-color are "vertex" and "edge"

	// Construct x-axis labels
	incr := (p.xmax - p.xmin) / (xlabels - 1)
	x := p.xmin
	// First label is empty for alignment purposes
	for i := range plot.Xlabel {
		plot.Xlabel[i] = fmt.Sprintf("%.2f", x)
		x += incr
	}

	// Construct the y-axis labels
	incr = (p.ymax - p.ymin) / (ylabels - 1)
	y := p.ymin
	for i := range plot.Ylabel {
		plot.Ylabel[i] = fmt.Sprintf("%.2f", y)
		y += incr
	}

	// Status
	if len(status) > 0 {
		plot.Status = strings.Join(status, ", ")
	}

	// Write to HTTP using template and grid
	if err := tmplForm.Execute(w, plot); err != nil {
		log.Fatalf("Write to HTTP output using template with grid error: %v\n", err)
	}

	return nil
}

// HTTP handler for /primmst connections
func handlePrimMST(w http.ResponseWriter, r *http.Request) {

	// Create the Prim MST instance
	primmst = &PrimMST{}

	// Accumulate error
	status := make([]string, 0)

	// Generate V vertices and locations randomly, get from HTML form
	// or read in from a previous graph when using a new start vertex.
	// Insert vertex complex coordinates into locations
	err := primmst.generateVertices(r)
	if err != nil {
		fmt.Printf("generateVertices error: %v", err)
		status = append(status, err.Error())
	}

	// Insert distances into graph
	err = primmst.findDistances()
	if err != nil {
		fmt.Printf("findDistances error: %v", err)
		status = append(status, err.Error())
	}

	// Find MST and save in PrimMST.mst
	err = primmst.findMST()
	if err != nil {
		fmt.Printf("findMST error: %v", err)
		status = append(status, err.Error())
	}

	// Draw MST into 300 x 300 cell 2px canvas
	// Construct x-axis labels, y-axis labels, status message
	err = primmst.plotMST(w, status)
	if err != nil {
		fmt.Printf("plotMST error: %v", err)
	}

}

// main sets up the http handlers, listens, and serves http clients
func main() {
	// Set up http server with handler for Prim MST execution
	http.HandleFunc(patternConnect, handlePrimMST)
	fmt.Printf("Prim MST Server listening on %v.\n", addr)
	http.ListenAndServe(addr, nil)
}
