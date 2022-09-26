import React from "react";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { PositionedEdge, PositionedNode } from "./flow-utils";
import * as ElkGraphLayoutData from "./algorithms/elk-algo";
import { FlowDiagram } from "./FlowDiagram";

jest.mock("./algorithms/elk-algo", () => ({
  __esModule: true,
  ...jest.requireActual("./algorithms/elk-algo"),
}));

jest.mock("@visx/responsive", () => ({
  ParentSize: (props: any) => {
    return props.children({
      width: 1000,
      height: 1000,
    });
  },
}));

const setMockLayoutData = (
  nodes: Partial<PositionedNode>[],
  edges: Partial<PositionedEdge> & { text: string }[] = []
) => {
  const mockCoordinates = { x: 0, y: 0 };
  const mockSize = {
    width: 100,
    height: 100,
  };
  const mockNodes = nodes.map((node) => {
    return {
      id: node.label,
      type: node.type || "service",
      ...mockSize,
      ...mockCoordinates,
      ...node,
    };
  });
  const mockEdges = edges.map((edge) => {
    return {
      id: (Math.random() + 1).toString(36).substring(7),
      label: {
        text: edge.text,
        ...mockCoordinates,
      },
      points: [mockCoordinates],
      ...edge,
    };
  });

  jest
    .spyOn(ElkGraphLayoutData, "getElkGraphLayoutData")
    .mockImplementationOnce(() => Promise.resolve({ nodes: mockNodes, edges: mockEdges }) as any);
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
        label: "service-1",
      },
      {
        label: "topic-1",
        type: "topic",
      },
    ]);
    render(<FlowDiagram metaData={getMetaDataMock()} />);

    await waitFor(() => {
      expect(screen.getByText("service-1")).toBeInTheDocument();
      expect(screen.getByText("topic-1")).toBeInTheDocument();
    });
  });

  it("should render edges and labels", async () => {
    setMockLayoutData(
      [
        {
          label: "service-1",
        },
        {
          label: "service-2",
        },
      ],
      [
        {
          type: "rpc",
          source: "service-1",
          target: "service-2",
          text: "3",
        },
      ]
    );
    const { container } = render(<FlowDiagram metaData={getMetaDataMock()} />);

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
          label: "service-1",
        },
        {
          label: "service-2",
        },
        {
          label: "service-3",
        },
      ],
      [
        {
          type: "rpc",
          source: "service-1",
          target: "service-2",
          text: "1",
        },
      ]
    );
    const { container } = render(<FlowDiagram metaData={getMetaDataMock()} />);

    await waitFor(() => {
      fireEvent.mouseEnter(screen.getByTestId("node-service-1"));

      // should now be visible
      const label = container.querySelector(".edge-group")!.querySelector(".label");
      expect(label!.classList).toContain("opacity-100");

      // should be dimmed
      expect(screen.getByTestId("node-service-3").classList).toContain("opacity-10");
    });
  });

  describe("service node info", () => {
    const getServiceNodeByName = (name: string) => {
      return screen.getByText(name).parentElement!.parentElement as HTMLElement;
    };

    it("should show database", async () => {
      setMockLayoutData([
        {
          label: "service-1",
          has_database: true,
        },
      ]);
      render(<FlowDiagram metaData={getMetaDataMock()} />);

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
          label: "service-1",
          cron_jobs: [{ title: "cron-job-title" }] as any,
        },
        {
          label: "service-2",
          cron_jobs: [{ title: "cron-job-1" }, { title: "cron-job-2" }] as any,
        },
      ]);
      render(<FlowDiagram metaData={getMetaDataMock()} />);

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
