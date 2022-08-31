import {
  getEdgesFromMetaData,
  getNodesFromMetaData,
} from "~lib/graph/graph-utils";
import { META_DATA_FIXTURE } from "../../__tests__/meta-data-fixture";

describe("Graph Utils", () => {
  describe("getNodesFromMetaData", () => {
    it("should nodes from meta data", () => {
      const nodes = getNodesFromMetaData(META_DATA_FIXTURE);

      expect(nodes).toHaveLength(4);

      const service = nodes.filter((n) => n.type === "service")[0];
      expect(service).toEqual({
        type: "service",
        id: "service:service-1",
        label: "service-1",
        service_name: "service-1",
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
