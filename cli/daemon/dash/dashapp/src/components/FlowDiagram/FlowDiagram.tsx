import React, { FC, useEffect, useState } from "react";
import {
  getEdgesFromMetaData,
  getNodesFromMetaData,
  GraphData,
  GraphLayoutOptions,
  NodeData,
  PositionedNode,
  ServiceNode,
} from "./flow-utils";
import { ProvidedZoom } from "@visx/zoom/lib/types";
import { getElkGraphLayoutData } from "./algorithms/elk-algo";
import { Group } from "@visx/group";
import { Zoom } from "@visx/zoom";
import {
  EdgeLabelSVG,
  EdgeSVG,
  EncoreArrowHeadSVG,
  ServiceSVG,
  TopicSVG,
} from "./flowSvgElements";
import { ParentSize } from "@visx/responsive";
import { APIMeta } from "~c/api/api";

class LayoutOptions implements GraphLayoutOptions {
  serviceNodeMinWidth = 220;
  topicNodeMinWidth = 50;
  serviceNodeMinHeight = 60;
  topicNodeMinHeight = 40;

  getNodeWidth(nodeData: NodeData) {
    const labelLen = nodeData.label.length;
    if (nodeData.type === "topic") {
      return Math.max(this.topicNodeMinWidth, labelLen * 9 + 60);
    }
    return Math.max(this.serviceNodeMinWidth, labelLen * 12);
  }

  getNodeHeight(nodeData: NodeData) {
    if (nodeData.type === "topic") return this.topicNodeMinHeight;
    if (nodeData.has_database) return this.serviceNodeMinHeight + 27;
    return this.serviceNodeMinHeight;
  }
}

interface Props {
  metaData: APIMeta;
}

const FlowDiagram: FC<Props> = ({ metaData }) => {
  const [graphLayoutData, setGraphLayoutData] = useState<GraphData>();
  const [activeNode, setActiveNode] = useState<PositionedNode | null>(null);
  const [activeDescendants, setActiveDescendants] = useState<string[] | null>(
    null
  );
  const [isInitialRender, setIsInitialRender] = useState<boolean>(true);
  const graphWidth = graphLayoutData?.width ?? 1600;
  const graphHeight = graphLayoutData?.height ?? 800;

  const getNumberOfEndpoints = (node: ServiceNode) => {
    const svc = metaData.svcs.find((s) => s.name === node.service_name);
    const endpoints = { public: 0, auth: 0, private: 0 };
    svc?.rpcs.forEach((rpc) => {
      if (rpc.access_type === "PUBLIC") endpoints.public++;
      if (rpc.access_type === "AUTH") endpoints.auth++;
      if (rpc.access_type === "PRIVATE") endpoints.private++;
    });
    return endpoints;
  };
  const getDescendantNodes = (node: NodeData) => {
    return graphLayoutData!.edges
      .filter((e) => e.source === node.id)
      .map((e) => e.target);
  };
  const isNodeActive = (node: NodeData) =>
    activeNode === null ||
    activeNode.id === node.id ||
    !!activeDescendants?.includes(node.id);
  const onNodeClick = (
    zoom: ProvidedZoom<SVGSVGElement>,
    node: PositionedNode
  ) => {
    const centerPoint = { x: graphWidth / 2, y: graphHeight / 2 };
    const inverseCentroid = zoom.applyInverseToPoint(centerPoint);
    zoom.translate({
      translateX: inverseCentroid.x - node.x,
      translateY: inverseCentroid.y - node.y,
    });
  };
  const onNodeMouseEnter = (node: PositionedNode) => {
    setActiveNode(node);
    setActiveDescendants(getDescendantNodes(node));
  };
  const onNodeMouseLeave = () => {
    setActiveNode(null);
    setActiveDescendants(null);
  };

  useEffect(() => {
    if (metaData) {
      getElkGraphLayoutData(
        getNodesFromMetaData(metaData),
        getEdgesFromMetaData(metaData),
        new LayoutOptions()
      ).then(setGraphLayoutData);
    }
  }, [metaData]);

  if (!graphLayoutData) return null;

  return (
    <div
      className="relative flex h-full w-full flex-col items-center justify-center"
      style={{ background: "#EEEEE1" }}
    >
      {!graphLayoutData.nodes.length ? (
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
            if (!parent.width || !parent.height) return null;

            return (
              <Zoom<SVGSVGElement>
                width={parent.width}
                height={parent.height}
                scaleXMin={0.5}
                scaleXMax={4}
                scaleYMin={0.5}
                scaleYMax={4}
                initialTransformMatrix={{
                  scaleX: 1,
                  scaleY: 1,
                  translateX: parent.width / 2 - graphLayoutData.width! / 2,
                  translateY: parent.height / 2 - graphLayoutData.height! / 2,
                  skewX: 0,
                  skewY: 0,
                }}
              >
                {(zoom) => (
                  <div className="relative">
                    <svg
                      id="flow-diagram"
                      width={parent.width}
                      height={parent.height}
                      ref={zoom.containerRef}
                    >
                      <defs>
                        <EncoreArrowHeadSVG id="encore-arrow" fill="#333" />
                      </defs>

                      {/* Background */}
                      <rect
                        fill="#EEEEE1"
                        width={parent.width}
                        height={parent.height}
                        onTouchStart={zoom.dragStart}
                        onTouchMove={zoom.dragMove}
                        onTouchEnd={zoom.dragEnd}
                        onMouseDown={zoom.dragStart}
                        onMouseMove={zoom.dragMove}
                        onMouseUp={zoom.dragEnd}
                        onMouseLeave={() => {
                          if (zoom.isDragging) zoom.dragEnd();
                        }}
                        style={{
                          cursor: zoom.isDragging ? "grabbing" : "grab",
                          touchAction: "none",
                        }}
                      />

                      {/* Drawable area */}
                      <Group transform={zoom.toString()}>
                        {graphLayoutData.edges.map((edge) => (
                          <Group key={edge.id} className="edge-group">
                            <EdgeSVG
                              edge={edge}
                              activeNodeId={activeNode?.id || null}
                              isInitialRender={isInitialRender}
                            />
                            <EdgeLabelSVG
                              edge={edge}
                              activeNodeId={activeNode?.id || null}
                            />
                          </Group>
                        ))}
                        {graphLayoutData.nodes.map((node) => {
                          if (node.type === "service")
                            return (
                              <ServiceSVG
                                key={node.id}
                                node={node}
                                isActive={isNodeActive(node)}
                                endpoints={getNumberOfEndpoints(node)}
                                onClick={onNodeClick.bind(null, zoom, node)}
                                onMouseEnter={onNodeMouseEnter.bind(null, node)}
                                onMouseLeave={onNodeMouseLeave.bind(null)}
                              />
                            );
                          if (node.type === "topic") {
                            return (
                              <TopicSVG
                                key={node.id}
                                node={node}
                                isActive={isNodeActive(node)}
                                onClick={onNodeClick.bind(null, zoom, node)}
                                onMouseEnter={onNodeMouseEnter.bind(null, node)}
                                onMouseLeave={onNodeMouseLeave.bind(null)}
                              />
                            );
                          }
                        })}
                      </Group>
                    </svg>
                  </div>
                )}
              </Zoom>
            );
          }}
        </ParentSize>
      )}
      <a
        target="_blank"
        href="https://encoredev.slack.com/app_redirect?channel=CQFNUESN9"
      >
        <p
          className="absolute left-2 bottom-2 p-2 text-sm font-semibold"
          style={{ background: "#EEEEE1" }}
        >
          Want to see more?
          <br />
          <span className="font-normal underline">
            Please share your feedback and ideas
          </span>
        </p>
      </a>
    </div>
  );
};

export default FlowDiagram;
