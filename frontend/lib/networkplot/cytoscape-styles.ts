/** Cytoscape stylesheet from networkplot html.py */
export const CYTOSCAPE_STYLES = [
  {
    selector: 'node[type = "layer"]',
    style: { display: "none", width: 1, height: 1, opacity: 0 },
  },
  {
    selector: "node",
    style: {
      shape: "round-rectangle",
      width: "label",
      height: "label",
      padding: "22px",
      "background-color": "data(card)",
      "border-width": 2,
      "border-color": "data(border)",
      label: "data(label)",
      "text-wrap": "wrap",
      "text-max-width": 200,
      "font-size": 13,
      "font-weight": 600,
      "text-valign": "bottom",
      "text-halign": "center",
      color: "data(textColor)",
      "background-image": "data(iconUrl)",
      "background-fit": "none",
      "background-width": "36px",
      "background-height": "36px",
      "background-position-x": "50%",
      "background-position-y": "10px",
      "background-repeat": "no-repeat",
      "text-margin-y": 10,
      "text-outline-width": 2,
      "text-outline-color": "#ffffff",
    },
  },
  {
    selector: 'node[type = "pod"]',
    style: {
      "font-size": 15,
      padding: "26px",
      "text-max-width": 220,
      "background-width": "42px",
      "background-height": "42px",
      "border-width": 3,
    },
  },
  {
    selector: 'node[directPod = "true"]',
    style: { "border-width": 4 },
  },
  {
    selector: 'node[namespaceOnly = "true"]',
    style: { "border-style": "dashed", "font-weight": 500, opacity: 0.88 },
  },
  {
    selector: "node:selected",
    style: { "border-width": 4 },
  },
  {
    selector: "edge",
    style: {
      width: 2.5,
      "target-arrow-shape": "triangle",
      "curve-style": "bezier",
      label: "data(label)",
      "font-size": 11,
      "font-weight": 700,
      "text-rotation": "autorotate",
      color: "#1e293b",
      "text-background-color": "#ffffff",
      "text-background-opacity": 0.92,
      "text-background-padding": 4,
    },
  },
  {
    selector: 'edge[edgeType = "direct"]',
    style: {
      "line-color": "#16a34a",
      "target-arrow-color": "#15803d",
      width: 3.5,
      color: "#14532d",
    },
  },
  {
    selector: 'edge[edgeType = "https"]',
    style: { "line-color": "#0d6efd", "target-arrow-color": "#0d6efd" },
  },
  {
    selector: 'edge[edgeType = "snat"]',
    style: {
      "line-color": "#fd7e14",
      "target-arrow-color": "#fd7e14",
      "line-style": "dashed",
    },
  },
  {
    selector: 'edge[edgeType = "scheduled"]',
    style: { "line-color": "#d97706", "target-arrow-color": "#d97706" },
  },
  {
    selector: 'edge[edgeType = "egressservice"]',
    style: {
      "line-color": "#7c3aed",
      "target-arrow-color": "#7c3aed",
      "line-style": "dashed",
    },
  },
  {
    selector: ".hl-edge",
    style: { width: 4, "z-index": 9 },
  },
  {
    selector: ".faded",
    style: { opacity: 0.18 },
  },
];
