import { getEdgesFromMetaData, getNodesFromMetaData } from "./flow-utils";
import { APIMeta } from "~c/api/api";

const META_DATA_FIXTURE = {
  pkgs: [
    {
      service_name: "service-1",
      rel_path: "service-1",
      rpc_calls: [{ pkg: "path/service-2", name: "MethodOnService2" }],
    },
    {
      service_name: "service-2",
      rel_path: "path/service-2",
      rpc_calls: [{ pkg: "path/service-3", name: "MethodOnService3" }],
    },
    {
      service_name: "",
      rel_path: "path",
      rpc_calls: [],
    },
    {
      service_name: "service-3",
      rel_path: "path/service-3",
      rpc_calls: [],
    },
  ],
  svcs: [
    {
      name: "service-1",
      rel_path: "service-1",
      rpcs: [],
      migrations: [],
      databases: ["service-1"],
    },
    {
      name: "service-2",
      rel_path: "path/service-2",
      rpcs: [{ name: "MethodOnService2" }],
      migrations: [],
      databases: ["service-3"],
    },
    {
      name: "service-3",
      rel_path: "path/service-3",
      rpcs: [{ name: "MethodOnService3" }],
      migrations: [],
      databases: [],
    },
  ] as any,
  pubsub_topics: [
    {
      name: "topic-1",
      subscriptions: [{ service_name: "service-1" }],
      publishers: [{ service_name: "service-2" }],
    },
  ] as any,
} as APIMeta;

describe("Graph Utils", () => {
  describe("getNodesFromMetaData", () => {
    it("should get nodes from meta data", () => {
      const nodes = getNodesFromMetaData(META_DATA_FIXTURE);

      expect(nodes).toHaveLength(4);

      const service = nodes.filter((n) => n.type === "service")[0];
      expect(service).toEqual({
        type: "service",
        id: "service:service-1",
        label: "service-1",
        service_name: "service-1",
        has_database: true,
      });

      const topic = nodes.filter((n) => n.type === "topic")[0];
      expect(topic).toEqual({
        id: "topic:topic-1",
        type: "topic",
        label: "topic-1",
      });
    });
  });

  describe("getEdgesFromMetaData", () => {
    it("should create edges for RPC calls", () => {
      const edges = getEdgesFromMetaData(META_DATA_FIXTURE).filter(
        (e) => e.type === "rpc"
      );

      expect(edges).toHaveLength(2);
      expect(edges[0]).toEqual({
        source: "service:service-1",
        target: "service:service-2",
        type: "rpc",
      });
      expect(edges[1]).toEqual({
        source: "service:service-2",
        target: "service:service-3",
        type: "rpc",
      });
    });

    it("should create edges for database dependencies", () => {
      const edges = getEdgesFromMetaData(META_DATA_FIXTURE).filter(
        (e) => e.type === "database"
      );

      expect(edges).toHaveLength(1);
      expect(edges[0]).toEqual({
        source: "service:service-2",
        target: "service:service-3",
        type: "database",
      });
    });

    it("should create edges for Pub/Sub topics", () => {
      const edges = getEdgesFromMetaData(META_DATA_FIXTURE).filter(
        (e) => e.type === "subscription" || e.type === "publish"
      );

      expect(edges).toHaveLength(2);
      expect(edges[0]).toEqual({
        source: "topic:topic-1",
        target: "service:service-1",
        type: "subscription",
      });
      expect(edges[1]).toEqual({
        source: "service:service-2",
        target: "topic:topic-1",
        type: "publish",
      });
    });
  });
});
