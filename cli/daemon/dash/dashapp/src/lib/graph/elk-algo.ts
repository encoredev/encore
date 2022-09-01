import {
  EdgeData,
  GetGraphLayoutData,
  GraphData,
  NodeData,
  PositionedEdge,
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
  const edgeMap = new Map<string, string>();
  const elkEdges: (ElkExtendedEdge & EdgeData)[] = [];
  edges.forEach((edgeData) => {
    const sourceTargetStr = `${edgeData.source}-${edgeData.target}`;

    // if we do not already have an edge of this type, then we add it
    if (edgeMap.get(sourceTargetStr) !== edgeData.type) {
      elkEdges.push({
        id: `${sourceTargetStr}:${edgeData.type}`,
        sources: [edgeData.source],
        targets: [edgeData.target],
        layoutOptions: {
          // priorities the topic edges, pushing the topics to the top of the graph
          "priority.direction": edgeData.source.match(/topic/) ? "10" : "1",
        },
        ...edgeData,
      });
      edgeMap.set(sourceTargetStr, edgeData.type);
    }
  });

  const elkNodes: (ElkNode & NodeData)[] = nodes.map((nodeData, index) => {
    return {
      layoutOptions: {},
      width: options.getNodeWidth(nodeData),
      height: options.getNodeHeight(nodeData),
      ...nodeData,
    };
  });

  const elk = new ELK({
    defaultLayoutOptions: {
      "elk.direction": "DOWN",
      "elk.edgeRouting": "POLYLINE",
      // add additional spacing between edges and nodes
      "org.eclipse.elk.spacing.edgeNode": "40",
    },
  });

  return elk
    .layout({ id: "id", children: elkNodes, edges: elkEdges })
    .then((graph) => {
      const { width, height } = graph;
      const children = graph.children as (ElkNode & NodeData)[];
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
    let points: { x: number; y: number }[] = [];
    (edge.sections || []).forEach((section) => {
      if (section.startPoint) points.push(section.startPoint);
      if (section.bendPoints) points = points.concat(section.bendPoints);
      if (section.endPoint) points = points.concat(section.endPoint);
    });
    return {
      ...edge,
      points,
    };
  });
};
