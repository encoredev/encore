import { getEdgesFromMetaData, getNodesFromMetaData, NodeData } from "./flow-utils";

const emptyMetaData = {
  cron_jobs: [],
  pkgs: [],
  svcs: [],
  pubsub_topics: [],
} as any;

describe("Graph Utils", () => {
  describe("getNodesFromMetaData", () => {
    it("should get services from meta data", () => {
      const nodes = getNodesFromMetaData({
        ...emptyMetaData,
        svcs: [
          {
            name: "service-1",
            databases: [],
          },
          {
            name: "service-2",
            databases: [],
          },
        ],
      });

      expect(nodes).toHaveLength(2);

      expect(nodes[0]).toEqual({
        type: "service",
        id: "service:service-1",
        labels: [{ text: "service-1" }],
        service_name: "service-1",
        has_database: false,
        cron_jobs: [],
        height: 57,
        width: 220,
      } as NodeData);
    });

    it("should get topics from meta data", () => {
      const nodes = getNodesFromMetaData({
        ...emptyMetaData,
        pubsub_topics: [
          {
            name: "topic-1",
          },
        ],
      });

      expect(nodes).toHaveLength(1);

      expect(nodes[0]).toEqual({
        id: "topic:topic-1",
        type: "topic",
        labels: [{ text: "topic-1" }],
        ports: [
          {
            id: "topic:topic-1:port",
          },
        ],
        height: 40,
        width: 123,
      } as NodeData);
    });

    it("should only return matching nodes if nodeID array is supplied", () => {
      const nodes = getNodesFromMetaData(
        {
          ...emptyMetaData,
          svcs: [
            {
              name: "service-1",
              databases: [],
            },
            {
              name: "service-2",
              databases: [],
            },
          ],
          pubsub_topics: [
            {
              name: "topic-1",
            },
          ],
        },
        ["service:service-1", "topic:topic-1:port"]
      );

      expect(nodes).toHaveLength(2);
    });

    it("should add database to service", () => {
      const nodes = getNodesFromMetaData({
        ...emptyMetaData,
        svcs: [
          {
            name: "service-name",
            databases: ["service-name"],
          },
        ],
      });

      expect(nodes[0]).toEqual(
        expect.objectContaining({
          has_database: true,
        })
      );
    });

    it("should add cron job to service", () => {
      const cronJobs = [
        { title: "cron-job-1", endpoint: { pkg: "service-1" } },
        { title: "cron-job-2", endpoint: { pkg: "path/service-2" } },
      ];
      const nodes = getNodesFromMetaData({
        ...emptyMetaData,
        cron_jobs: cronJobs,
        pkgs: [
          {
            service_name: "service-1",
            rel_path: "service-1",
          },
          {
            service_name: "service-2",
            rel_path: "path/service-2",
          },
        ],
        svcs: [
          {
            name: "service-1",
            databases: [],
          },
          {
            name: "service-2",
            databases: [],
          },
        ],
      });

      expect(nodes[0]).toEqual(
        expect.objectContaining({
          cron_jobs: [cronJobs[0]],
        })
      );

      expect(nodes[1]).toEqual(
        expect.objectContaining({
          cron_jobs: [cronJobs[1]],
        })
      );
    });
  });

  describe("getEdgesFromMetaData", () => {
    it("should create edges for RPC calls", () => {
      const edges = getEdgesFromMetaData({
        ...emptyMetaData,
        pkgs: [
          {
            service_name: "service-1",
            rel_path: "service-1",
            rpc_calls: [{ pkg: "path/service-2" }],
          },
          {
            service_name: "service-2",
            rel_path: "path/service-2",
            rpc_calls: [{ pkg: "path/service-3" }],
          },
          {
            service_name: "",
            rel_path: "path",
            rpc_calls: [],
          },
          {
            service_name: "service-3",
            rel_path: "path/service-3",
            // this RPC call should be filtered out, not external call
            rpc_calls: [{ pkg: "path/service-3" }],
          },
        ],
      }).filter((e) => e.type === "rpc");

      expect(edges).toHaveLength(2);
      expect(edges[0]).toEqual({
        id: "service:service-1-service:service-2:rpc",
        sources: ["service:service-1"],
        targets: ["service:service-2"],
        type: "rpc",
      });
      expect(edges[1]).toEqual({
        id: "service:service-2-service:service-3:rpc",
        sources: ["service:service-2"],
        targets: ["service:service-3"],
        type: "rpc",
      });
    });

    it("should create edges for database dependencies", () => {
      const edges = getEdgesFromMetaData({
        ...emptyMetaData,
        svcs: [
          {
            name: "service-1",
            rel_path: "service-1",
            databases: ["service-2"],
          },
          {
            name: "service-2",
            rel_path: "path/service-2",
            databases: ["service-2"],
          },
        ],
      }).filter((e) => e.type === "database");

      expect(edges).toHaveLength(1);
      expect(edges[0]).toEqual({
        id: "service:service-1-service:service-2:database",
        sources: ["service:service-1"],
        targets: ["service:service-2"],
        type: "database",
      });
    });

    it("should create edges for Pub/Sub topics", () => {
      const edges = getEdgesFromMetaData({
        ...emptyMetaData,
        pubsub_topics: [
          {
            name: "topic-1",
            subscriptions: [{ service_name: "service-1" }],
            publishers: [{ service_name: "service-2" }],
          },
        ],
      }).filter((e) => e.type === "subscription" || e.type === "publish");

      expect(edges).toHaveLength(2);
      expect(edges[0]).toEqual({
        id: "topic:topic-1-service:service-1:subscription",
        sources: ["topic:topic-1"],
        targets: ["service:service-1"],
        type: "subscription",
      });
      expect(edges[1]).toEqual({
        id: "service:service-2-topic:topic-1:publish",
        sources: ["service:service-2"],
        targets: ["topic:topic-1:port"],
        type: "publish",
      });
    });

    it("should return edges connected to node if targetNodeID is supplied", () => {
      const edges = getEdgesFromMetaData(
        {
          ...emptyMetaData,
          pkgs: [
            {
              service_name: "service-1",
              rel_path: "service-1",
              rpc_calls: [{ pkg: "path/service-2" }],
            },
            {
              service_name: "service-2",
              rel_path: "path/service-2",
              rpc_calls: [],
            },
          ],
          pubsub_topics: [
            {
              name: "topic-1",
              subscriptions: [{ service_name: "service-1" }],
              publishers: [{ service_name: "service-2" }],
            },
          ],
        },
        "service:service-2"
      );

      expect(edges).toHaveLength(2);
      expect(edges[0]).toEqual({
        id: "service:service-1-service:service-2:rpc",
        sources: ["service:service-1"],
        targets: ["service:service-2"],
        type: "rpc",
      });
      expect(edges[1]).toEqual({
        id: "service:service-2-topic:topic-1:publish",
        sources: ["service:service-2"],
        targets: ["topic:topic-1:port"],
        type: "publish",
      });
    });
  });
});
