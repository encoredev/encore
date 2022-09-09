import { APIMeta } from "~c/api/api";

export const META_DATA_FIXTURE = {
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
