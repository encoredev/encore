import React from "react";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import * as FlowUtils from "./flow-utils";
import * as ElkGraphLayoutData from "./algorithms/elk-algo";
import { FlowDiagram } from "./FlowDiagram";

jest.mock("./algorithms/elk-algo", () => ({
  __esModule: true,
  ...jest.requireActual("./algorithms/elk-algo"),
  getCoordinatePointsForEdge: (edge: FlowUtils.EdgeData) => ({ ...edge, points: [{ x: 0, y: 0 }] }),
}));

jest.mock("./flow-utils", () => ({
  __esModule: true,
  ...jest.requireActual("./flow-utils"),
}));

jest.mock("@visx/responsive", () => ({
  ParentSize: (props: any) => {
    return props.children({
      width: 1000,
      height: 1000,
    });
  },
}));

jest.mock("panzoom", () => ({
  __esModule: true,
  ...jest.requireActual("./algorithms/elk-algo"),
  default: () => ({
    zoomAbs: () => ({}),
    moveTo: () => ({}),
  }),
}));

const setMockLayoutData = (
  nodes: Partial<FlowUtils.NodeData>[],
  edges: Partial<FlowUtils.EdgeData>[] = []
) => {
  const mockCoordinates = { x: 0, y: 0 };
  const mockSize = {
    width: 100,
    height: 100,
  };
  const mockNodes = nodes.map((node) => {
    return {
      id: node.labels![0].text,
      type: node.type || "service",
      ...mockSize,
      ...mockCoordinates,
      ...node,
    };
  });
  const mockEdges = edges.map((edge) => {
    return {
      id: (Math.random() + 1).toString(36).substring(7),
      ...edge,
    };
  });

  jest
    .spyOn(ElkGraphLayoutData, "getElkAppGraphLayoutData")
    .mockImplementationOnce(
      () => Promise.resolve({ children: mockNodes, edges: mockEdges }) as any
    );
};

const getMetaDataMock = (metaData: any = {}) => {
  return { svcs: [], pubsub_topics: [], pkgs: [], cron_jobs: [], ...metaData };
};

describe("FlowDiagram", () => {
  afterEach(() => {
    jest.clearAllMocks();
    jest.resetAllMocks();
    jest.restoreAllMocks();
    cleanup();
  });

  it("should render services and topics", async () => {
    setMockLayoutData([
      {
        labels: [{ text: "service-1" }],
      },
      {
        labels: [{ text: "topic-1" }],
        type: "topic",
      },
    ]);
    render(<FlowDiagram metaData={getMetaDataMock()} onChangeDetailedViewNode={jest.fn} />);

    await waitFor(() => {
      expect(screen.getByText("service-1")).toBeInTheDocument();
      expect(screen.getByText("topic-1")).toBeInTheDocument();
    });
  });

  it("should render edges and labels", async () => {
    setMockLayoutData(
      [
        {
          labels: [{ text: "service-1" }],
        },
        {
          labels: [{ text: "service-2" }],
        },
      ],
      [
        {
          type: "rpc",
          sources: ["service-1"],
          targets: ["service-2"],
          labels: [{ text: "3" }],
        },
      ]
    );
    const { container } = render(
      <FlowDiagram metaData={getMetaDataMock()} onChangeDetailedViewNode={jest.fn} />
    );

    await waitFor(() => {
      const edgeEl = container.querySelector(".edge");
      expect(edgeEl).toBeTruthy();
      expect(screen.getByText("3 RPCs")).toBeInTheDocument();
    });
  });

  it("should highlight descendants when hovering node", async () => {
    setMockLayoutData(
      [
        {
          labels: [{ text: "service-1" }],
        },
        {
          labels: [{ text: "service-2" }],
        },
        {
          labels: [{ text: "service-3" }],
        },
      ],
      [
        {
          type: "rpc",
          sources: ["service-1"],
          targets: ["service-2"],
          labels: [{ text: "1" }],
        },
      ]
    );
    const { container } = render(
      <FlowDiagram metaData={getMetaDataMock()} onChangeDetailedViewNode={jest.fn} />
    );

    await waitFor(() => {
      fireEvent.mouseEnter(screen.getByTestId("node-service-1"));
    });

    // should now be visible
    const label = container.querySelector(".edge-label-group")!.querySelector(".label");
    expect(label!.classList).toContain("opacity-100");

    // should be dimmed
    expect(screen.getByTestId("node-service-3").classList).toContain("opacity-10");
  });

  it("should show detailed view of node if props is specified", async () => {
    const getElkAppGraphLayoutDataSpy = jest.spyOn(ElkGraphLayoutData, "getElkAppGraphLayoutData");
    const getNodesFromMetaDataSpy = jest.spyOn(FlowUtils, "getNodesFromMetaData");
    const getEdgesFromMetaDataSpy = jest.spyOn(FlowUtils, "getEdgesFromMetaData");
    setMockLayoutData(
      [
        {
          labels: [{ text: "service-1" }],
          name: "service-1",
        },
        {
          labels: [{ text: "service-2" }],
          name: "service-2",
        },
        {
          labels: [{ text: "service-3" }],
          name: "service-3",
        },
      ],
      [
        {
          type: "rpc",
          sources: ["service-1"],
          targets: ["service-2"],
          labels: [{ text: "1" }],
        },
      ]
    );
    render(
      <FlowDiagram
        metaData={getMetaDataMock()}
        detailedViewNode="service-1"
        onChangeDetailedViewNode={jest.fn()}
      />
    );

    await waitFor(() => {
      expect(getNodesFromMetaDataSpy).toHaveBeenLastCalledWith(getMetaDataMock(), [
        "service-1",
        "service-2",
      ]);
      expect(getEdgesFromMetaDataSpy).toHaveBeenLastCalledWith(getMetaDataMock(), "service-1");
      expect(getElkAppGraphLayoutDataSpy).toHaveBeenLastCalledWith([], [], {
        "elk.direction": "DOWN",
      });
    });
  });

  it.only("should show detailed view of service when clicking on a node", async () => {
    const getElkAppGraphLayoutDataSpy = jest.spyOn(ElkGraphLayoutData, "getElkAppGraphLayoutData");
    const getNodesFromMetaDataSpy = jest.spyOn(FlowUtils, "getNodesFromMetaData");
    const getEdgesFromMetaDataSpy = jest.spyOn(FlowUtils, "getEdgesFromMetaData");
    const onChangeServiceDetailedView = jest.fn();
    setMockLayoutData(
      [
        {
          labels: [{ text: "service-1" }],
          name: "service-1",
        },
        {
          labels: [{ text: "service-2" }],
          name: "service-2",
        },
        {
          labels: [{ text: "service-3" }],
          name: "service-3",
        },
      ],
      [
        {
          type: "rpc",
          sources: ["service-1"],
          targets: ["service-2"],
          labels: [{ text: "1" }],
        },
      ]
    );
    render(
      <FlowDiagram
        metaData={getMetaDataMock()}
        onChangeDetailedViewNode={onChangeServiceDetailedView}
      />
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId("node-service-1"));
    });

    await waitFor(() => {
      expect(onChangeServiceDetailedView).toBeCalledWith("service-1");
    });
  });

  describe("service node info", () => {
    const getServiceNodeByName = (name: string) => {
      return screen.getByText(name).parentElement!.parentElement as HTMLElement;
    };

    it("should show database", async () => {
      setMockLayoutData([
        {
          labels: [{ text: "service-1" }],
          has_database: true,
        },
      ]);
      render(<FlowDiagram metaData={getMetaDataMock()} onChangeDetailedViewNode={jest.fn} />);

      await waitFor(() => {
        expect(within(getServiceNodeByName("service-1")).getByText("Database")).toBeInTheDocument();
      });
    });

    it("should show endpoints", async () => {
      render(
        <FlowDiagram
          metaData={getMetaDataMock({
            svcs: [
              {
                name: "service-1",
                rpcs: [
                  { access_type: "PUBLIC" },
                  { access_type: "AUTH" },
                  { access_type: "PRIVATE" },
                ],
                databases: [],
              },
            ],
          })}
          onChangeDetailedViewNode={jest.fn}
        />
      );

      await waitFor(() => {
        const serviceNode1 = getServiceNodeByName("service-1");

        expect(within(serviceNode1).getByTestId("service-endpoints")).toHaveTextContent("1 public");
        expect(within(serviceNode1).getByTestId("service-endpoints")).toHaveTextContent("1 auth");
        expect(within(serviceNode1).getByTestId("service-endpoints")).toHaveTextContent(
          "1 private"
        );
      });
    });

    it("should show cron jobs", async () => {
      setMockLayoutData([
        {
          labels: [{ text: "service-1" }],
          cron_jobs: [{ title: "cron-job-title" }] as any,
        },
        {
          labels: [{ text: "service-2" }],
          cron_jobs: [{ title: "cron-job-1" }, { title: "cron-job-2" }] as any,
        },
      ]);
      render(<FlowDiagram metaData={getMetaDataMock()} onChangeDetailedViewNode={jest.fn} />);

      await waitFor(() => {
        expect(
          within(getServiceNodeByName("service-1")).getByTestId("service-cron-jobs")
        ).toHaveTextContent("cron-job-title");
        expect(
          within(getServiceNodeByName("service-2")).getByTestId("service-cron-jobs")
        ).toHaveTextContent("2 cron jobs");
      });
    });
  });
});
