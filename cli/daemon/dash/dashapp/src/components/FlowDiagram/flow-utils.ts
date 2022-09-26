import { APIMeta, CronJob } from "~c/api/api";

export const OUTSIDE_FACING_NODE_ID = ":outside-facing:";

export interface Coordinates {
  x: number;
  y: number;
}

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
  has_database: boolean;
  cron_jobs: CronJob[];
}

export interface TopicNode extends BaseNode {
  type: "topic";
}

export type NodeData = ServiceNode | TopicNode;

export type PositionedNode<T = ServiceNode | TopicNode> = T &
  Coordinates & {
    width: number;
    height: number;
  };

export type PositionedEdge = EdgeData & {
  id: string;
  points: Coordinates[];
  label?: Coordinates & { text: string };
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

const getPackageToServiceMap = (metaData: APIMeta) => {
  const map = new Map<string, string>();
  metaData.pkgs.forEach((pkg) => {
    if (pkg.service_name) {
      map.set(pkg.rel_path, pkg.service_name);
    }
  });
  return map;
};

const getServiceToPackageMap = (metaData: APIMeta) => {
  const map = new Map<string, string[]>();
  metaData.pkgs.forEach((pkg) => {
    if (pkg.service_name) {
      const paths = map.get(pkg.service_name) ?? [];
      map.set(pkg.service_name, [...paths, pkg.rel_path]);
    }
  });
  return map;
};

export const getNodesFromMetaData = (metaData: APIMeta) => {
  const nodes: NodeData[] = [];
  const svcMap = getServiceToPackageMap(metaData);

  // Services
  metaData.svcs.forEach((svc) => {
    // clone cron jobs array
    const cronJobs = metaData.cron_jobs.map((cronJob) =>
      Object.assign({}, cronJob)
    );

    nodes.push({
      id: serviceID(svc.name),
      label: svc.name,
      type: "service",
      service_name: svc.name,
      has_database: svc.databases.some((dbName) => dbName === svc.name),
      cron_jobs: cronJobs.filter((cronJob) => {
        const pkgsForSvc = svcMap.get(svc.name);
        return pkgsForSvc && pkgsForSvc.includes(cronJob.endpoint.pkg);
      }),
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
