import React, { FC, useEffect, useState } from "react";
import { APIMeta } from "~c/api/api";
import JSONRPCConn from "~lib/client/jsonrpc";
import { Group } from "@visx/group";
import { Zoom } from "@visx/zoom";
import {
  getEdgesFromMetaData,
  getNodesFromMetaData,
  GraphData,
  PositionedNode,
  ServiceNode,
} from "~lib/graph/graph-utils";
import {
  EdgeSVG,
  EncoreArrowHeadSVG,
  ServiceSVG,
  TopicSVG,
} from "~c/svgElements";
import { ProvidedZoom } from "@visx/zoom/lib/types";
import { getElkGraphLayoutData } from "~lib/graph/elk-algo";

interface Props {
  appID: string;
  conn: JSONRPCConn;
}

const AppDiagram: FC<Props> = ({ appID, conn }) => {
  const [metaData, setMetaData] = useState<APIMeta>();
  const [graphLayoutData, setGraphLayoutData] = useState<GraphData>();
  const hasServiceDatabase = (node: ServiceNode) => {
    const svc = metaData!.svcs.find((s) => s.name === node.service_name);
    return !!svc?.databases.some((dbName) => dbName === svc.name);
  };
  const isServiceOutsideFacing = (node: ServiceNode) => {
    const svc = metaData!.svcs.find((s) => s.name === node.service_name);
    return !!svc?.rpcs.some(
      (rpc) => rpc.access_type === "PUBLIC" || rpc.access_type === "AUTH"
    );
  };
  const graphPadding = 20;
  const width = graphLayoutData?.width
    ? graphLayoutData.width + graphPadding * 2
    : 1600;
  const height = graphLayoutData?.height
    ? graphLayoutData?.height + graphPadding * 2
    : 800;
  const initialZoomTransform = {
    scaleX: 1,
    scaleY: 1,
    translateX: graphPadding,
    translateY: graphPadding,
    skewX: 0,
    skewY: 0,
  };

  useEffect(() => {
    conn.request("status", { appID }).then((status: any) => {
      if (status.meta) {
        setMetaData(status.meta);
      }
    });
  }, []);

  useEffect(() => {
    const nodeMinWidth = 150;
    const serviceNodeHeight = 80;
    const topicNodeHeight = 40;
    if (metaData) {
      getElkGraphLayoutData(
        getNodesFromMetaData(metaData),
        getEdgesFromMetaData(metaData),
        {
          getNodeWidth: (nodeData) => {
            const labelLen = nodeData.label.length;
            return Math.max(nodeMinWidth, labelLen * 12);
          },
          getNodeHeight: (nodeData) => {
            return nodeData.type === "topic"
              ? topicNodeHeight
              : serviceNodeHeight;
          },
          // drawOutsideDependencyToNode: (nodeData) => {
          //   return (
          //     nodeData.type === "service" && isServiceOutsideFacing(nodeData)
          //   );
          // },
        }
      ).then((data) => setGraphLayoutData(data));
    }
  }, [metaData]);

  const getNodeElement = (
    zoom: ProvidedZoom<SVGSVGElement>,
    node: PositionedNode
  ) => {
    const onNodeClick = () => {
      const centerPoint = { x: width / 2, y: height / 2 };
      const inverseCentroid = zoom.applyInverseToPoint(centerPoint);
      zoom.translate({
        translateX: inverseCentroid.x - node.x,
        translateY: inverseCentroid.y - node.y,
      });
    };
    switch (node.type) {
      case "service":
        return (
          <ServiceSVG
            key={node.id}
            node={node}
            hasDatabase={hasServiceDatabase(node)}
            isOutsideFacing={isServiceOutsideFacing(node)}
            onClick={onNodeClick}
          />
        );
      case "topic":
        return <TopicSVG key={node.id} node={node} onClick={onNodeClick} />;
      default:
        return null;
    }
  };

  if (!graphLayoutData) return null;

  return (
    <div className="flex flex-col">
      <Zoom<SVGSVGElement>
        width={width}
        height={height}
        scaleXMin={0.5}
        scaleXMax={4}
        scaleYMin={0.5}
        scaleYMax={4}
        initialTransformMatrix={initialZoomTransform}
      >
        {(zoom) => (
          <div className="relative">
            <svg
              width={width}
              height={height}
              style={{
                cursor: zoom.isDragging ? "grabbing" : "grab",
                touchAction: "none",
              }}
              ref={zoom.containerRef}
            >
              <defs>
                <EncoreArrowHeadSVG id="encore-arrow" fill="#333" />
              </defs>

              {/* Background */}
              <rect
                width={width}
                height={height}
                fill="#FFF"
                onTouchStart={zoom.dragStart}
                onTouchMove={zoom.dragMove}
                onTouchEnd={zoom.dragEnd}
                onMouseDown={zoom.dragStart}
                onMouseMove={zoom.dragMove}
                onMouseUp={zoom.dragEnd}
                onMouseLeave={() => {
                  if (zoom.isDragging) zoom.dragEnd();
                }}
              />

              {/* Drawable area */}
              <Group transform={zoom.toString()}>
                {graphLayoutData.nodes.map(getNodeElement.bind(null, zoom))}
                {graphLayoutData.edges.map((edge, index) => (
                  <EdgeSVG
                    key={`${edge.source}-${edge.target}-${index}`}
                    edge={edge}
                  />
                ))}
              </Group>
            </svg>
          </div>
        )}
      </Zoom>
    </div>
  );
};

export default AppDiagram;
