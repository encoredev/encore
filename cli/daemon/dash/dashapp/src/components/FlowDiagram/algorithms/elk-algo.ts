import { EdgeData, NodeData, PositionedEdge } from "../flow-utils";
import ELK from "elkjs/lib/elk.bundled.js";
import { ElkPoint } from "elkjs/lib/elk-api";
import { LayoutOptions } from "elkjs";

/**
 * https://github.com/kieler/elkjs
 */
export const getElkInfraGraphLayoutData = (nodes: NodeData[]): Promise<NodeData> => {
  return new ELK().layout(
    {
      id: "infra-graph",
      children: nodes,
    },
    {
      layoutOptions: {
        // The 'INCLUDE_CHILDREN' option is required if we want to draw edges through nodes (which
        // we want if we want to show infra dependencies) but the drawback is that the layout gets
        // worse if you enable it.
        // https://github.com/eclipse/elk/issues/490#issuecomment-584002943
        // hierarchyHandling: "INCLUDE_CHILDREN",
        "elk.nodeLabels.placement": "V_TOP H_LEFT INSIDE",
        "elk.padding": "[left=10, top=35, right=10, bottom=10]",
      },
    }
  ) as Promise<NodeData>;
};

/**
 * https://github.com/kieler/elkjs
 */
export const getElkAppGraphLayoutData = (
  nodes: NodeData[],
  edges: EdgeData[],
  layoutOptions: LayoutOptions
): Promise<NodeData> => {
  const edgeTypeMap = new Map<string, number>();
  let elkEdges: EdgeData[] = [];
  edges.forEach((edge) => {
    // if we do not already have an edge of this type, then we add it
    if (!edgeTypeMap.get(edge.id)) {
      elkEdges.push({
        layoutOptions: {
          // priorities the topic edges, pushing the topics to the top of the graph
          // "priority.direction": edge.sources.some((s) => s.match(/topic/)) ? "10" : "1",
        },
        ...edge,
      });
      edgeTypeMap.set(edge.id, 1);
    } else {
      const prev = edgeTypeMap.get(edge.id)!;
      edgeTypeMap.set(edge.id, prev + 1);
    }
  });

  elkEdges = elkEdges.map((edge) => {
    const calls = edgeTypeMap.get(edge.id);
    return {
      labels: [{ text: calls?.toString() }],
      ...edge,
    };
  });

  // Options: https://www.eclipse.org/elk/reference.html
  const elk = new ELK({
    defaultLayoutOptions: {
      "elk.edgeRouting": "ORTHOGONAL",
      "org.eclipse.elk.spacing.portPort": "20",
      "org.eclipse.elk.spacing.edgeNode": "20", // add spacing between edges and nodes
      "org.eclipse.elk.edgeLabels.placement": "HEAD", // place labels near the edge "target"
      "org.eclipse.elk.spacing.edgeLabel": "20", // add spacing between edges and label
      "org.eclipse.elk.edgeLabels.inline": "true", // edge label is placed directly on its edge
      "org.eclipse.elk.layered.edgeLabels.sideSelection": "SMART_DOWN", // deciding on edge label sides
      ...layoutOptions,
      // "elk.direction": "RIGHT",
      // "org.eclipse.elk.spacing.edgeEdge": "100",
      // "org.eclipse.elk.spacing.portsSurrounding": "[left=500, top=0, right=500, bottom=0]",
      // "org.eclipse.elk.spacing.labelPortHorizontal": "10",
      // "org.eclipse.elk.layered.spacing.edgeEdgeBetweenLayers": "100",
      // "org.eclipse.elk.spacing.nodeNode": "200",
    },
  });

  return elk.layout({ id: "app-graph", children: nodes, edges: elkEdges }) as Promise<NodeData>;
};

export const getCoordinatePointsForEdge = (edge: EdgeData): PositionedEdge => {
  let points: ElkPoint[] = [];
  (edge.sections || []).forEach((section) => {
    if (section.startPoint) points.push(section.startPoint);
    if (section.bendPoints) points = points.concat(section.bendPoints);
    if (section.endPoint) points = points.concat(section.endPoint);
  });
  return {
    ...edge,
    points,
  };
};
