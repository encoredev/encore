import * as dagre from "dagre";
import {
  GetGraphLayoutData,
  NodeData,
  OUTSIDE_FACING_NODE_ID,
  PositionedEdge,
  PositionedNode,
} from "~lib/graph/graph-utils";

/**
 * Directed graph layout
 * https://github.com/dagrejs/dagre/wiki
 */
export const getGraphLayoutData: GetGraphLayoutData = (
  nodes,
  edges,
  options
) => {
  const graphData = new dagre.graphlib.Graph<NodeData>();

  graphData.setGraph({
    // nodesep: 100, // number of pixels that separate nodes horizontally in the layout.
    // ranksep: 60, // number of pixels between each rank in the layout.
    // ranker: 'network-simplex',
    ranker: "longest-path", // longer lines but less overlap over nodes
  });

  graphData.setDefaultEdgeLabel(() => ({}));

  if (options.drawOutsideDependencyToNode) {
    graphData.setNode(OUTSIDE_FACING_NODE_ID, {
      type: "service",
      id: OUTSIDE_FACING_NODE_ID,
      width: 0,
      height: 0,
    });
  }

  nodes.forEach((nodeData) => {
    graphData.setNode(nodeData.id, {
      ...nodeData,
      width: options.getNodeWidth(nodeData),
      height: options.getNodeHeight(nodeData),
    });
    if (
      options.drawOutsideDependencyToNode &&
      options.drawOutsideDependencyToNode(nodeData)
    ) {
      graphData.setEdge(OUTSIDE_FACING_NODE_ID, nodeData.id, { weight: 10000 });
    }
  });

  edges.forEach((edgeData) => {
    graphData.setEdge(edgeData.source, edgeData.target, edgeData);
  });

  dagre.layout(graphData, {});

  const positionNodes: PositionedNode[] = graphData
    .nodes()
    .map((n) => graphData.node(n));
  const positionedEdges: PositionedEdge[] = graphData
    .edges()
    .map((e) => graphData.edge(e) as PositionedEdge);

  return Promise.resolve({ nodes: positionNodes, edges: positionedEdges });
};
