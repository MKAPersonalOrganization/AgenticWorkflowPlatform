// Package main demonstrates visualizing knowledge graphs as interactive HTML.
//
// This example shows how to:
// 1. Export the knowledge graph to GraphML format
// 2. Parse the GraphML and extract nodes and edges
// 3. Generate an interactive HTML visualization using vis.js
//
// The output is a standalone HTML file that can be opened in any browser.
package main

import (
	"encoding/xml"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// GraphML structures for parsing
type GraphML struct {
	XMLName xml.Name `xml:"graphml"`
	Graph   Graph    `xml:"graph"`
}

type Graph struct {
	Nodes []Node `xml:"node"`
	Edges []Edge `xml:"edge"`
}

type Node struct {
	ID   string `xml:"id,attr"`
	Data []Data `xml:"data"`
}

type Edge struct {
	Source string `xml:"source,attr"`
	Target string `xml:"target,attr"`
	Data   []Data `xml:"data"`
}

type Data struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}

// VisNode represents a node for vis.js visualization
type VisNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Title string `json:"title"`
	Group string `json:"group"`
}

// VisEdge represents an edge for vis.js visualization
type VisEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
	Title string `json:"title"`
}

// HTMLData contains data for the HTML template
type HTMLData struct {
	Title string
	Nodes []VisNode
	Edges []VisEdge
}

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <script type="text/javascript" src="https://unpkg.com/vis-network/standalone/umd/vis-network.min.js"></script>
    <style type="text/css">
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 0;
            background-color: #1a1a2e;
            color: #eee;
        }
        #header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            padding: 20px;
            text-align: center;
            box-shadow: 0 2px 10px rgba(0,0,0,0.3);
        }
        #header h1 {
            margin: 0;
            font-size: 24px;
        }
        #header p {
            margin: 5px 0 0;
            opacity: 0.8;
            font-size: 14px;
        }
        #network {
            width: 100%;
            height: calc(100vh - 120px);
            border: none;
        }
        #stats {
            position: fixed;
            bottom: 20px;
            right: 20px;
            background: rgba(0,0,0,0.7);
            padding: 15px;
            border-radius: 8px;
            font-size: 14px;
        }
        #controls {
            position: fixed;
            bottom: 20px;
            left: 20px;
            background: rgba(0,0,0,0.7);
            padding: 15px;
            border-radius: 8px;
        }
        button {
            background: #667eea;
            border: none;
            color: white;
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            margin: 2px;
        }
        button:hover {
            background: #764ba2;
        }
    </style>
</head>
<body>
    <div id="header">
        <h1>{{.Title}}</h1>
        <p>Knowledge Graph Visualization</p>
    </div>
    <div id="network"></div>
    <div id="stats">
        <strong>Statistics</strong><br>
        Nodes: {{len .Nodes}}<br>
        Edges: {{len .Edges}}
    </div>
    <div id="controls">
        <button onclick="network.fit()">Fit View</button>
        <button onclick="togglePhysics()">Toggle Physics</button>
    </div>

    <script type="text/javascript">
        var nodes = new vis.DataSet([
            {{range .Nodes}}
            {id: "{{.ID}}", label: "{{.Label}}", title: "{{.Title}}", group: "{{.Group}}"},
            {{end}}
        ]);

        var edges = new vis.DataSet([
            {{range .Edges}}
            {from: "{{.From}}", to: "{{.To}}", label: "{{.Label}}", title: "{{.Title}}"},
            {{end}}
        ]);

        var container = document.getElementById('network');
        var data = {
            nodes: nodes,
            edges: edges
        };

        var options = {
            nodes: {
                shape: 'dot',
                size: 20,
                font: {
                    size: 14,
                    color: '#ffffff'
                },
                borderWidth: 2,
                shadow: true
            },
            edges: {
                width: 2,
                color: {color: '#848484', highlight: '#667eea'},
                font: {
                    size: 12,
                    color: '#aaaaaa',
                    strokeWidth: 0
                },
                arrows: {
                    to: {enabled: true, scaleFactor: 0.5}
                },
                smooth: {
                    type: 'continuous'
                }
            },
            groups: {
                'PERSON': {color: {background: '#e74c3c', border: '#c0392b'}},
                'ORGANIZATION': {color: {background: '#3498db', border: '#2980b9'}},
                'CONCEPT': {color: {background: '#2ecc71', border: '#27ae60'}},
                'LOCATION': {color: {background: '#f39c12', border: '#d68910'}},
                'EVENT': {color: {background: '#9b59b6', border: '#8e44ad'}},
                'default': {color: {background: '#95a5a6', border: '#7f8c8d'}}
            },
            physics: {
                enabled: true,
                barnesHut: {
                    gravitationalConstant: -2000,
                    centralGravity: 0.3,
                    springLength: 150,
                    springConstant: 0.04,
                    damping: 0.09
                },
                stabilization: {
                    iterations: 150,
                    fit: true
                }
            },
            interaction: {
                hover: true,
                tooltipDelay: 200,
                hideEdgesOnDrag: true
            }
        };

        var network = new vis.Network(container, data, options);
        var physicsEnabled = true;

        function togglePhysics() {
            physicsEnabled = !physicsEnabled;
            network.setOptions({physics: {enabled: physicsEnabled}});
        }

        network.on("click", function(params) {
            if (params.nodes.length > 0) {
                var nodeId = params.nodes[0];
                console.log("Clicked node:", nodeId);
            }
        });
    </script>
</body>
</html>
`

func main() {
	// Configure logging
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Check command line arguments
	if len(os.Args) < 2 {
		// Use a demo if no file provided
		logger.Info("No GraphML file provided, generating demo visualization")
		generateDemo(logger)
		return
	}

	graphMLPath := os.Args[1]
	outputPath := strings.TrimSuffix(graphMLPath, filepath.Ext(graphMLPath)) + ".html"

	if len(os.Args) >= 3 {
		outputPath = os.Args[2]
	}

	logger.Infof("Input: %s", graphMLPath)
	logger.Infof("Output: %s", outputPath)

	// Read and parse GraphML file
	data, err := os.ReadFile(graphMLPath)
	if err != nil {
		logger.Fatalf("Failed to read GraphML file: %v", err)
	}

	var graphml GraphML
	if err := xml.Unmarshal(data, &graphml); err != nil {
		logger.Fatalf("Failed to parse GraphML: %v", err)
	}

	logger.Infof("Found %d nodes and %d edges", len(graphml.Graph.Nodes), len(graphml.Graph.Edges))

	// Convert to vis.js format
	visNodes := make([]VisNode, 0, len(graphml.Graph.Nodes))
	visEdges := make([]VisEdge, 0, len(graphml.Graph.Edges))

	for _, node := range graphml.Graph.Nodes {
		visNode := VisNode{
			ID:    node.ID,
			Label: node.ID,
			Group: "default",
		}

		var description string
		for _, d := range node.Data {
			switch d.Key {
			case "entity_type", "type":
				visNode.Group = d.Value
			case "description":
				description = d.Value
			case "label", "name":
				visNode.Label = d.Value
			}
		}

		visNode.Title = fmt.Sprintf("%s\\n%s", visNode.Label, description)
		visNodes = append(visNodes, visNode)
	}

	for _, edge := range graphml.Graph.Edges {
		visEdge := VisEdge{
			From: edge.Source,
			To:   edge.Target,
		}

		var description string
		for _, d := range edge.Data {
			switch d.Key {
			case "description", "label":
				visEdge.Label = truncate(d.Value, 30)
				description = d.Value
			case "keywords":
				if visEdge.Label == "" {
					visEdge.Label = truncate(d.Value, 30)
				}
			}
		}

		visEdge.Title = description
		visEdges = append(visEdges, visEdge)
	}

	// Generate HTML
	if err := generateHTML(outputPath, "Knowledge Graph", visNodes, visEdges); err != nil {
		logger.Fatalf("Failed to generate HTML: %v", err)
	}

	logger.Infof("Visualization saved to: %s", outputPath)
	logger.Info("Open the file in a web browser to view the interactive graph")
}

// generateDemo creates a sample visualization without a GraphML file
func generateDemo(logger *logrus.Logger) {
	outputPath := "./knowledge_graph_demo.html"

	// Sample nodes
	nodes := []VisNode{
		{ID: "einstein", Label: "Albert Einstein", Group: "PERSON", Title: "Theoretical physicist who developed relativity"},
		{ID: "relativity", Label: "Theory of Relativity", Group: "CONCEPT", Title: "Scientific theory of space and time"},
		{ID: "curie", Label: "Marie Curie", Group: "PERSON", Title: "Pioneer in radioactivity research"},
		{ID: "radioactivity", Label: "Radioactivity", Group: "CONCEPT", Title: "Emission of ionizing radiation"},
		{ID: "nobel", Label: "Nobel Prize", Group: "EVENT", Title: "International scientific award"},
		{ID: "physics", Label: "Physics", Group: "CONCEPT", Title: "Natural science studying matter and energy"},
		{ID: "chemistry", Label: "Chemistry", Group: "CONCEPT", Title: "Science of matter and its properties"},
	}

	// Sample edges
	edges := []VisEdge{
		{From: "einstein", To: "relativity", Label: "developed", Title: "Einstein developed the theory of relativity"},
		{From: "einstein", To: "nobel", Label: "received", Title: "Einstein received the Nobel Prize in 1921"},
		{From: "einstein", To: "physics", Label: "studied", Title: "Einstein was a physicist"},
		{From: "curie", To: "radioactivity", Label: "discovered", Title: "Curie discovered radioactivity"},
		{From: "curie", To: "nobel", Label: "received twice", Title: "Curie received Nobel Prize in physics and chemistry"},
		{From: "curie", To: "physics", Label: "studied", Title: "Curie studied physics"},
		{From: "curie", To: "chemistry", Label: "studied", Title: "Curie also studied chemistry"},
		{From: "relativity", To: "physics", Label: "part of", Title: "Relativity is a branch of physics"},
		{From: "radioactivity", To: "physics", Label: "related to", Title: "Radioactivity is studied in physics"},
	}

	if err := generateHTML(outputPath, "Demo Knowledge Graph", nodes, edges); err != nil {
		logger.Fatalf("Failed to generate demo HTML: %v", err)
	}

	logger.Infof("Demo visualization saved to: %s", outputPath)
	logger.Info("Open the file in a web browser to view the interactive graph")
}

func generateHTML(outputPath, title string, nodes []VisNode, edges []VisEdge) error {
	tmpl, err := template.New("graph").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	data := HTMLData{
		Title: title,
		Nodes: nodes,
		Edges: edges,
	}

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
