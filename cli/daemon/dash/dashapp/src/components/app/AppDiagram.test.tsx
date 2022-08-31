import React from "react";
import { render, screen, waitFor, within } from "@testing-library/react";
import AppDiagram from "~c/app/AppDiagram";
import { META_DATA_FIXTURE } from "../../__tests__/meta-data-fixture";

describe("AppDiagram", () => {
  const connMock = {
    request: () => Promise.resolve({ meta: META_DATA_FIXTURE }),
  };

  it("should render nodes in diagram", async () => {
    render(<AppDiagram appID={"app-id"} conn={connMock as any} />);

    await waitFor(() => {
      expect(screen.getByText("service-1")).toBeInTheDocument();
      expect(screen.getByText("service-2")).toBeInTheDocument();
      expect(screen.getByText("service-3")).toBeInTheDocument();
      expect(screen.getByText("topic-1")).toBeInTheDocument();
    });
  });

  it("should add database for service nodes", async () => {
    render(<AppDiagram appID={"app-id"} conn={connMock as any} />);

    await waitFor(() => {
      const serviceNode1 = screen.getByText("service-1")
        .parentElement as HTMLElement;
      const serviceNode2 = screen.getByText("service-2")
        .parentElement as HTMLElement;

      expect(
        within(serviceNode1).getByTestId("service-database")
      ).toBeInTheDocument();
      expect(() =>
        // Will throw because it can't find the element
        within(serviceNode2).getByTestId("service-database")
      ).toThrow();
    });
  });
});
