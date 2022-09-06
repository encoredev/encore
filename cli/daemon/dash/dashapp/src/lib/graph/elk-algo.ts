import {
  Coordinates,
  EdgeData,
  GetGraphLayoutData,
  GraphData,
  NodeData,
  PositionedEdge,
  PositionedNode,
} from "~lib/graph/graph-utils";
import ELK, { ElkNode } from "elkjs/lib/elk.bundled.js";
import { ElkExtendedEdge } from "elkjs/lib/elk-api";

/**
 * https://github.com/kieler/elkjs
 */
export const getElkGraphLayoutData: GetGraphLayoutData = (
  nodes,
  edges,
  options
) => {
  const edgeTypeMap = new Map<string, number>();
  let elkEdges: (ElkExtendedEdge & EdgeData)[] = [];
  edges.forEach((edge) => {
    // can only have one edge of a particular type for a source to a target
    const id = `${edge.source}-${edge.target}:${edge.type}`;

    // if we do not already have an edge of this type, then we add it
    if (!edgeTypeMap.get(id)) {
      elkEdges.push({
        id,
        sources: [edge.source],
        targets: [edge.target],
        layoutOptions: {
          // priorities the topic edges, pushing the topics to the top of the graph
          "priority.direction": edge.source.match(/topic/) ? "10" : "1",
        },
        ...edge,
      });
      edgeTypeMap.set(id, 1);
    } else {
      const prev = edgeTypeMap.get(id)!;
      edgeTypeMap.set(id, prev + 1);
    }
  });

  elkEdges = elkEdges.map((edge) => {
    const calls = edgeTypeMap.get(edge.id);
    return {
      labels: [{ text: calls?.toString() }],
      ...edge,
    };
  });

  const elkNodes: (ElkNode & NodeData)[] = nodes.map((nodeData, index) => {
    return {
      layoutOptions: {},
      width: options.getNodeWidth(nodeData),
      height: options.getNodeHeight(nodeData),
      ...nodeData,
    };
  });

  // Options: https://www.eclipse.org/elk/reference.html
  const elk = new ELK({
    defaultLayoutOptions: {
      "elk.direction": "DOWN", // arrows are mostly pointing downwards
      "elk.edgeRouting": "POLYLINE",
      "org.eclipse.elk.spacing.edgeNode": "40", // add spacing between edges and nodes
      "org.eclipse.elk.edgeLabels.placement": "HEAD", // place labels near the edge "target"
      "org.eclipse.elk.spacing.edgeLabel": "20", // add spacing between edges and label
      "org.eclipse.elk.edgeLabels.inline": "true", // edge label is placed directly on its edge
      "org.eclipse.elk.layered.edgeLabels.sideSelection": "SMART_DOWN", // deciding on edge label sides
    },
  });

  return elk
    .layout({ id: "add-diagram", children: elkNodes, edges: elkEdges })
    .then((graph) => {
      const { width, height } = graph;
      const children = graph.children as (ElkNode & PositionedNode)[];
      const edges = getEdgesWithCoordinatePoints(
        graph.edges as (ElkExtendedEdge & EdgeData)[]
      );
      return {
        nodes: children,
        edges,
        width,
        height,
      } as GraphData;
    });
};

const getEdgesWithCoordinatePoints = (
  edges: (ElkExtendedEdge & EdgeData)[] = []
): PositionedEdge[] => {
  return edges.map((edge) => {
    let points: Coordinates[] = [];
    (edge.sections || []).forEach((section) => {
      if (section.startPoint) points.push(section.startPoint);
      if (section.bendPoints) points = points.concat(section.bendPoints);
      if (section.endPoint) points = points.concat(section.endPoint);
    });
    const label = edge.labels?.length ? edge.labels[0] : undefined;
    return {
      ...edge,
      label,
      points,
    } as PositionedEdge;
  });
};
