import { APIMeta, CronJob } from "~c/api/api";
import { ElkExtendedEdge, ElkNode, ElkPoint } from "elkjs/lib/elk-api";

// Can not show infra graph right now in local dev dash but having this so that we can copy and paste Flow changes
// between the repos
export type GetInfraResourcesQuery = any;

export interface EdgeData extends ElkExtendedEdge {
  type: "rpc" | "subscription" | "publish" | "database";
}

export interface PositionedEdge extends EdgeData {
  points: ElkPoint[];
}

interface BaseNode extends ElkNode {
  type: "service" | "topic" | "infra";
  children?: NodeData[];
  edges?: EdgeData[];
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

export interface InfraNode extends BaseNode {
  type: "infra";
  typename: GetInfraResourcesQuery["app"]["env"]["infraResources"][0]["__typename"];
}

export type NodeData = ServiceNode | TopicNode | InfraNode;

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

const getServiceNodeWidth = (label: string) => {
  const serviceNodeMinWidth = 220;
  return Math.max(serviceNodeMinWidth, label.length * 12);
};

const getServiceNodeHeight = (hasDatabase: boolean, cronJobs: number) => {
  const serviceNodeMinHeight = 57;
  let height = serviceNodeMinHeight;
  if (hasDatabase) height += 29;
  if (cronJobs) height += 29;
  return height;
};

const getTopicNodeWidth = (label: string) => {
  const minWidth = 50;
  const maxWidth = 300;
  return Math.min(Math.max(minWidth, label.length * 9 + 60), maxWidth);
};

const getTopicNodeHeight = () => {
  const topicNodeMinHeight = 40;
  return topicNodeMinHeight;
};

export const getNodesFromMetaData = (metaData: APIMeta, nodesToShow: string[] = []) => {
  const nodes: NodeData[] = [];
  const svcMap = getServiceToPackageMap(metaData);

  // Services
  metaData.svcs.forEach((svc) => {
    const cronJobs = metaData.cron_jobs
      // clone cron jobs array
      .map((cronJob) => Object.assign({}, cronJob))
      .filter((cronJob) => {
        const pkgsForSvc = svcMap.get(svc.name);
        return pkgsForSvc && pkgsForSvc.includes(cronJob.endpoint.pkg);
      });
    const hasDatabase = svc.databases.some((dbName) => dbName === svc.name);

    nodes.push({
      id: serviceID(svc.name),
      labels: [{ text: svc.name }],
      type: "service",
      service_name: svc.name,
      has_database: hasDatabase,
      cron_jobs: cronJobs,
      width: getServiceNodeWidth(svc.name),
      height: getServiceNodeHeight(hasDatabase, cronJobs.length),
    });
  });

  // Pub/Sub
  metaData.pubsub_topics.forEach((topic) => {
    nodes.push({
      id: topicID(topic.name),
      labels: [{ text: topic.name }],
      type: "topic",
      ports: [{ id: topicID(topic.name) + ":port" }],
      width: getTopicNodeWidth(topic.name),
      height: getTopicNodeHeight(),
    });
  });

  return nodesToShow.length
    ? nodes.filter(
        (node) =>
          nodesToShow.find((n) => n === node.id) ||
          (!!node.ports?.length && !!nodesToShow?.includes(node.ports[0].id))
      )
    : nodes;
};

export const getEdgesFromMetaData = (metaData: APIMeta, targetNodeID?: string) => {
  const edges: EdgeData[] = [];
  const pkgMap = getPackageToServiceMap(metaData);

  // Service RPC calls
  metaData.pkgs.forEach((pkg) => {
    const selfSvc = pkg.service_name;
    if (!selfSvc) return;

    pkg.rpc_calls.forEach((call) => {
      const serviceName = pkgMap.get(call.pkg);
      if (serviceName && serviceName !== selfSvc) {
        const source = serviceID(selfSvc);
        const target = serviceID(serviceName);
        const type = "rpc";
        edges.push({
          id: `${source}-${target}:${type}`,
          sources: [source],
          targets: [target],
          type,
        });
      }
    });
  });

  // Database dependencies
  metaData.svcs.forEach((svc) => {
    const dbUses = svc.databases.filter((dbName) => dbName !== svc.name);
    dbUses.forEach((dbName) => {
      const source = serviceID(svc.name);
      const target = serviceID(dbName); // DB name === svc name
      const type = "database";
      edges.push({
        id: `${source}-${target}:${type}`,
        sources: [source],
        targets: [target],
        type,
      });
    });
  });

  // Pub/Sub
  metaData.pubsub_topics.forEach((topic) => {
    topic.subscriptions.forEach((sub) => {
      const source = topicID(topic.name);
      const target = serviceID(sub.service_name);
      const type = "subscription";
      edges.push({
        id: `${source}-${target}:${type}`,
        sources: [source],
        targets: [target],
        type,
      });
    });
    topic.publishers.forEach((pub) => {
      const source = serviceID(pub.service_name);
      const target = topicID(topic.name);
      const type = "publish";
      edges.push({
        id: `${source}-${target}:${type}`,
        sources: [source],
        targets: [target + ":port"],
        type,
      });
    });
  });

  return targetNodeID
    ? edges.filter((edge) => {
        const edgeConnections = [...edge.sources, ...edge.targets];
        return (
          edgeConnections.includes(targetNodeID) || edgeConnections.includes(targetNodeID + ":port")
        );
      })
    : edges;
};

export const getNodesFromInfraData = (infraData: GetInfraResourcesQuery): NodeData[] => {
  const { infraResources } = infraData.app.env;
  const usedResourcesIDs = new Set<string>();
  // for excludedGroups we want to exclude all children of that parent group type
  const excludedGroups = ["*encoreplatform.EncorePlatform"];
  const excludedTypes = [
    "*gcp.APIAccess",
    "*gcp.IAMBinding",
    "*gcp.IAM",
    "*gcp.CloudRunToCloudRunLink",
    "*gcp.GoogleManagedServiceAccount",
    "*gcp.ServiceAccount",
    "*gcp.Service",
    "*gcp.RedisKeyspace",
    "*resource.RuntimeConfig",
    "*resource.ServiceConfig",
    "*resource.Model",
    "*gcp.Organization",
    "*gcp.Secrets",
    "*gcp.Secret",
    "*gcp.DockerImage",
    "*resource.AppSecrets",
    "*grafana.Metric",
    "*gcp.CloudRunToGrafanaCloudLink",
    "*gcp.CloudRunToTopicLink",
    "*gcp.TopicToCloudRunLink",
    "*gcp.CloudRunToRedisLink",
    "*gcp.CloudRunToCloudSQLLink",
    "*gcp.SQLServerUser",
    "*gcp.SQLSslCert",
    "*gcp.VPCPeeringRange",
    "*resource.AppConfig",
    "*grafana.Cloud",
    "*encoreplatform.CronJob",
    "*encoreplatform.DefaultRoute",
    ...excludedGroups,
  ];

  // const getEdgeData = (source: string, target: string): (ElkExtendedEdge & EdgeData)[] => {
  //   const resource = infraResources.find((el) => el.data.id === target);
  //   if (resource && !exclude.includes(resource.data.type)) {
  //     return [
  //       {
  //         id: `${source}-${target}`,
  //         sources: [source],
  //         targets: [target],
  //         type: "rpc",
  //       },
  //     ];
  //   }
  //   return [];
  // };

  const isPartOfExcludedGroup = (resourceID: string | null | undefined): boolean => {
    if (!resourceID) return false;
    const r = infraResources.find((el: any) => el.data.id === resourceID)!;
    if (!r) return false;
    return excludedGroups.includes(r.data.type) || isPartOfExcludedGroup(r.data.parent);
  };

  const getNodeData = (resourceID: string): NodeData[] => {
    if (!usedResourcesIDs.has(resourceID)) {
      const resource = infraResources.find((el: any) => el.data.id === resourceID)!;
      usedResourcesIDs.add(resourceID);
      if (!excludedTypes.includes(resource.data.type) && !isPartOfExcludedGroup(resourceID)) {
        const labelType = resource.data.label || resource.data.type || resource.__typename;
        let labelName: string | null | undefined = "";
        if (
          resource.__typename == "PubSubTopic" ||
          resource.__typename == "PubSubSubscription" ||
          resource.__typename == "CacheCluster" ||
          resource.__typename == "SQLDatabase"
        ) {
          labelName = resource.encoreName;
        }
        if (resource.__typename == "GCPCloudRun") labelName = resource.name;
        const label = labelName ? `${labelType} - ${labelName}` : labelType;
        // const edges = resource.data.dependencies.flatMap((depID) => getEdgeData(resourceID, depID));
        return [
          {
            id: resourceID,
            type: "infra",
            typename: resource.__typename,
            edges: [],
            children: resource.data.children.flatMap(getNodeData),
            labels: [{ text: label }],
            width: label.length * 10 + 40,
            height: 40,
          },
        ];
      }
    }
    return [];
  };

  return infraResources.flatMap((resource: any) => {
    return getNodeData(resource.data.id);
  });
};
