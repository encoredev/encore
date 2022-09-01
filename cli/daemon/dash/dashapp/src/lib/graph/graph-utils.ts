import { APIMeta } from "~c/api/api";

export const OUTSIDE_FACING_NODE_ID = ":outside-facing:";

export interface EdgeData {
  source: string;
  target: string;
  type: "rpc" | "subscription" | "publish" | "database";
}

interface BaseNode {
  id: string;
  label: string;
  type: "service" | "topic";
}

export interface ServiceNode extends BaseNode {
  type: "service";
  service_name: string;
}

export interface TopicNode extends BaseNode {
  type: "topic";
}

export type NodeData = ServiceNode | TopicNode;

export type PositionedNode = NodeData & {
  x: number;
  y: number;
  width: number;
  height: number;
};

export type PositionedEdge = EdgeData & {
  points: { x: number; y: number }[];
};

export interface GraphData {
  edges: PositionedEdge[];
  nodes: PositionedNode[];
  width?: number;
  height?: number;
}

export interface GraphLayoutOptions {
  getNodeWidth: (node: NodeData) => number;
  getNodeHeight: (node: NodeData) => number;
  drawOutsideDependencyToNode?: (node: NodeData) => boolean;
}

export interface GetGraphLayoutData {
  (
    nodes: NodeData[],
    edges: EdgeData[],
    options: GraphLayoutOptions
  ): Promise<GraphData>;
}

const serviceID = (svcName: string) => {
  return `service:${svcName}`;
};

const topicID = (topicName: string) => {
  return `topic:${topicName}`;
};

export const getNodesFromMetaData = (metaData: APIMeta) => {
  console.log(metaData);
  const nodes: NodeData[] = [];

  // Services
  metaData.svcs.forEach((svc) => {
    nodes.push({
      id: serviceID(svc.name),
      label: svc.name,
      type: "service",
      service_name: svc.name,
    });
  });

  // Pub/Sub
  metaData.pubsub_topics.forEach((topic) => {
    nodes.push({
      id: topicID(topic.name),
      label: topic.name,
      type: "topic",
    });
  });

  return nodes;
};

const getPackageToServiceMap = (metaData: APIMeta) => {
  const map = new Map<string, string>();
  metaData.pkgs.forEach((pkg) => {
    if (pkg.service_name) {
      map.set(pkg.rel_path, pkg.service_name);
    }
  });
  return map;
};

export const getEdgesFromMetaData = (metaData: APIMeta) => {
  const edges: Omit<EdgeData, "points">[] = [];

  const pkgMap = getPackageToServiceMap(metaData);

  // Service RPC calls
  metaData.pkgs.forEach((pkg) => {
    const selfSvc = pkg.service_name;
    if (!selfSvc) return;

    pkg.rpc_calls.forEach((call) => {
      const serviceName = pkgMap.get(call.pkg);
      if (serviceName && serviceName !== selfSvc) {
        edges.push({
          source: serviceID(selfSvc),
          target: serviceID(serviceName),
          type: "rpc",
        });
      }
    });
  });

  // Database dependencies
  metaData.svcs.forEach((svc) => {
    const dbUses = svc.databases.filter((dbName) => dbName !== svc.name);
    dbUses.forEach((dbName) => {
      edges.push({
        source: serviceID(svc.name),
        target: serviceID(dbName), // DB name === svc name
        type: "database",
      });
    });
  });

  // Pub/Sub
  metaData.pubsub_topics.forEach((topic) => {
    topic.subscriptions.forEach((sub) => {
      edges.push({
        source: topicID(topic.name),
        target: serviceID(sub.service_name),
        type: "subscription",
      });
    });
    topic.publishers.forEach((pub) => {
      edges.push({
        source: serviceID(pub.service_name),
        target: topicID(topic.name),
        type: "publish",
      });
    });
  });

  return edges;
};
