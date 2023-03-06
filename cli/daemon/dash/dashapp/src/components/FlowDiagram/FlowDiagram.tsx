import React, { FC, useEffect, useRef, useState } from "react";
import {
  getEdgesFromMetaData,
  GetInfraResourcesQuery,
  getNodesFromInfraData,
  getNodesFromMetaData,
  InfraNode,
  NodeData,
  ServiceNode,
} from "./flow-utils";
import {
  getCoordinatePointsForEdge,
  getElkAppGraphLayoutData,
  getElkInfraGraphLayoutData,
} from "./algorithms/elk-algo";
import { Group } from "@visx/group";
import {
  EdgeLabelSVG,
  EdgeSVG,
  EncoreArrowHeadSVG,
  InfraSVG,
  ServiceSVG,
  TopicSVG,
} from "./flowSvgElements";
import { ParentSize } from "@visx/responsive";
import { APIMeta } from "~c/api/api";
import useDownloadDiagram from "~c/FlowDiagram/useDownloadDiagram";
import Button from "~c/Button";
import { icons } from "~c/icons";
import panzoom, { PanZoom } from "panzoom";
import { ArrowSmallLeftIcon } from "@heroicons/react/24/outline";

const getAppLayoutData = (
  metaData: APIMeta,
  nodeID?: string,
  nodesIDs?: string[]
): Promise<NodeData> => {
  return getElkAppGraphLayoutData(
    getNodesFromMetaData(metaData, nodesIDs),
    getEdgesFromMetaData(metaData, nodeID),
    {
      "elk.direction": nodeID ? "DOWN" : "RIGHT",
    }
  );
};

function usePanzoom() {
  const panZoomRef = useRef<PanZoom | null>(null);
  const ref = React.useCallback((element: SVGSVGElement | null) => {
    if (!element) return;
    panZoomRef.current = panzoom(element, {
      smoothScroll: false,
      maxZoom: 5,
      minZoom: 0.2,
    });

    return () => panZoomRef.current?.dispose();
  }, []);

  return {
    ref,
    panZoomRef,
  };
}

interface Props {
  metaData?: APIMeta;
  infraData?: GetInfraResourcesQuery;
  serviceDetailedView?: string;
  onChangeServiceDetailedView?: (serviceName?: string) => void;
}

export const FlowDiagram: FC<Props> = ({
  metaData,
  infraData,
  serviceDetailedView,
  onChangeServiceDetailedView,
}) => {
  const [completeNodeData, setCompleteNodeData] = useState<NodeData>();
  const [displayNodeData, setDisplayNodeData] = useState<NodeData>();
  const [hoveringNode, setHoveringNode] = useState<NodeData | null>(null);
  const [activeDependencies, setActiveDependencies] = useState<string[] | null>(null);
  const [detailedViewNode, setDetailedViewNode] = useState<NodeData | null>(null);
  const [isInitialRender, setIsInitialRender] = useState<boolean>(true);
  const [isLoadingScreenshot, setIsLoadingScreenshot] = useState<boolean>(false);
  const screenshotRef = useRef<HTMLDivElement>(null);
  const downloadDiagram = useDownloadDiagram(screenshotRef);
  const { ref: zoomRef, panZoomRef } = usePanzoom();

  const onDownloadScreenshot = () => {
    setIsLoadingScreenshot(true);
    // putting this in a timeout to ensure that the loading animation
    // starts before the browser is blocked
    setTimeout(async () => {
      await downloadDiagram();
      setIsLoadingScreenshot(false);
    }, 50);
  };

  const getNumberOfEndpoints = (node: ServiceNode) => {
    const svc = metaData?.svcs.find((s) => s.name === node.service_name);
    const endpoints = { public: 0, auth: 0, private: 0 };
    svc?.rpcs.forEach((rpc) => {
      if (rpc.access_type === "PUBLIC") endpoints.public++;
      if (rpc.access_type === "AUTH") endpoints.auth++;
      if (rpc.access_type === "PRIVATE") endpoints.private++;
    });
    return endpoints;
  };

  const getOutboundDependencies = (node: NodeData): string[] => {
    return (completeNodeData!.edges ?? [])
      .filter((e) => e.sources[0] === node.id)
      .flatMap((e) => e.targets);
  };

  const getInboundDependencies = (node: NodeData): string[] => {
    return (completeNodeData!.edges ?? [])
      .filter((e) => {
        return (
          e.targets[0] === node.id || (!!node.ports?.length && e.targets[0] === node.ports[0].id)
        );
      })
      .flatMap((e) => e.sources);
  };

  const isNodeActive = (node: NodeData) => {
    if (hoveringNode === null || hoveringNode.id === node.id) return true;
    if (detailedViewNode) return true;
    return (
      !!activeDependencies?.includes(node.id) ||
      // check if active dependencies includes port
      (!!node.ports?.length && !!activeDependencies?.includes(node.ports[0].id))
    );
  };

  const onNodeClick = (node: NodeData) => {
    setDetailedViewNode(node);
    if (node.type === "service" && onChangeServiceDetailedView) {
      onChangeServiceDetailedView(node.service_name);
    }
  };
  getOutboundDependencies;
  const onNodeMouseEnter = (node: NodeData) => {
    setHoveringNode(node);
    setActiveDependencies(getOutboundDependencies(node));
  };

  const onNodeMouseLeave = () => {
    setHoveringNode(null);
    setActiveDependencies(null);
  };

  const getInfraNodeElement = (node: InfraNode) => {
    return (
      <InfraSVG
        key={node.id}
        node={node}
        isActive={isNodeActive(node)}
        onMouseEnter={onNodeMouseEnter.bind(null, node)}
        onMouseLeave={onNodeMouseLeave.bind(null)}
      >
        {!!node.children?.length &&
          node.children.flatMap((n) => getInfraNodeElement(n as InfraNode))}
      </InfraSVG>
    );
  };

  useEffect(() => {
    if (metaData) {
      getAppLayoutData(metaData).then((data) => {
        setCompleteNodeData(data);
        setDisplayNodeData(data);
      });
    }
    if (infraData) {
      getElkInfraGraphLayoutData(getNodesFromInfraData(infraData)).then((data) => {
        setCompleteNodeData(data);
        setDisplayNodeData(data);
      });
    }
  }, [metaData, infraData]);

  useEffect(() => {
    const serviceNode = completeNodeData?.children?.find((node) => {
      if (node.type === "service" && serviceDetailedView) {
        return node.service_name === serviceDetailedView;
      }
    });
    setDetailedViewNode(serviceNode ?? null);
  }, [serviceDetailedView, completeNodeData]);

  useEffect(() => {
    const nodeIDs: string[] = [];
    if (detailedViewNode) {
      nodeIDs.push(
        detailedViewNode.id,
        ...getOutboundDependencies(detailedViewNode),
        ...getInboundDependencies(detailedViewNode)
      );
    }
    if (metaData) {
      getAppLayoutData(metaData, detailedViewNode?.id, nodeIDs).then((data) => {
        setDisplayNodeData(data);
      });
    }
  }, [detailedViewNode, metaData]);

  if (!displayNodeData) return null;

  return (
    <div
      className="relative flex h-full w-full flex-col items-center justify-center"
      style={{ background: "#EEEEE1" }}
    >
      {!displayNodeData.children?.length ? (
        <div>
          <p>Add an service to your app and it will show up here</p>
        </div>
      ) : (
        <ParentSize>
          {(parent) => {
            useEffect(() => {
              if (parent.width && parent.height) {
                setIsInitialRender(false);
              }
            }, [parent]);

            useEffect(() => {
              const scaleX = parent.width / displayNodeData.width!;
              const scaleY = parent.height / displayNodeData.height!;
              let scale = scaleY > scaleX ? scaleX : scaleY;
              scale = Math.min(scale, 1.5) * 0.9;

              panZoomRef.current?.zoomAbs(0, 0, scale);
              panZoomRef.current?.moveTo(
                parent.width / 2 - (displayNodeData.width! * scale) / 2,
                parent.height / 2 - (displayNodeData.height! * scale) / 2
              );
            }, [parent.width, parent.height, displayNodeData.width, displayNodeData.height]);

            if (!parent.width || !parent.height) return null;

            return (
              <div className="relative h-full w-full overflow-hidden" ref={screenshotRef}>
                <svg
                  id="flow-diagram"
                  width={displayNodeData.width}
                  height={displayNodeData.height}
                  ref={zoomRef}
                >
                  <defs>
                    <EncoreArrowHeadSVG id="encore-arrow" fill="#333" />
                  </defs>

                  {/* Drawable area */}
                  <Group className="select-none">
                    {(displayNodeData.edges ?? []).map((edge) => (
                      <Group key={edge.id} className="edge-group">
                        <EdgeSVG
                          edge={getCoordinatePointsForEdge(edge)}
                          isInitialRender={isInitialRender}
                          isActive={
                            detailedViewNode
                              ? true
                              : hoveringNode === null || hoveringNode?.id === edge.sources[0]
                          }
                        />
                      </Group>
                    ))}
                    {(displayNodeData.children ?? []).map((node) => {
                      if (node.type === "service") {
                        return (
                          <ServiceSVG
                            key={node.id}
                            node={node}
                            shouldAnimateReposition={!detailedViewNode}
                            isActive={isNodeActive(node)}
                            endpoints={getNumberOfEndpoints(node)}
                            onClick={onNodeClick.bind(null, node)}
                            onMouseEnter={onNodeMouseEnter.bind(null, node)}
                            onMouseLeave={onNodeMouseLeave.bind(null)}
                          />
                        );
                      }
                      if (node.type === "topic") {
                        return (
                          <TopicSVG
                            key={node.id}
                            node={node}
                            shouldAnimateReposition={!detailedViewNode}
                            isActive={isNodeActive(node)}
                            onClick={onNodeClick.bind(null, node)}
                            onMouseEnter={onNodeMouseEnter.bind(null, node)}
                            onMouseLeave={onNodeMouseLeave.bind(null)}
                          />
                        );
                      }
                      if (node.type === "infra") {
                        return getInfraNodeElement(node);
                      }
                    })}

                    {(displayNodeData.edges ?? []).map((edge) => (
                      <Group key={edge.id} className="edge-label-group">
                        <EdgeLabelSVG
                          edge={getCoordinatePointsForEdge(edge)}
                          isActive={
                            detailedViewNode?.id === edge.sources[0] ||
                            hoveringNode?.id === edge.sources[0]
                          }
                        />
                      </Group>
                    ))}
                  </Group>
                </svg>
              </div>
            );
          }}
        </ParentSize>
      )}
      {detailedViewNode && (
        <div className="absolute left-2 top-2">
          <Button
            kind="primary"
            onClick={() => {
              setDetailedViewNode(null);
              if (onChangeServiceDetailedView) {
                onChangeServiceDetailedView(undefined);
              }
            }}
          >
            <ArrowSmallLeftIcon className="mr-2 h-4 w-4" />
            <p>View full application</p>
          </Button>
        </div>
      )}

      <div className="absolute right-2 top-2">
        <Button kind="primary" onClick={onDownloadScreenshot}>
          {isLoadingScreenshot ? (
            icons.loading("h-6 w-6", "#EEEEE1", "transparent", 4)
          ) : (
            <p>Share</p>
          )}
        </Button>
      </div>
    </div>
  );
};
