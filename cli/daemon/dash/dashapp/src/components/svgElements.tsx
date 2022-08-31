import * as React from "react";
import { SVGProps } from "react";
import { Group } from "@visx/group";
import { Circle, LinePath } from "@visx/shape";
import { PositionedEdge, PositionedNode } from "~lib/graph/graph-utils";
import { curveCatmullRom } from "@visx/curve";

export const ServiceSVG = ({
  node,
  hasDatabase,
  isOutsideFacing,
  onClick,
}: {
  node: PositionedNode;
  hasDatabase: boolean;
  isOutsideFacing: boolean;
  onClick: (event: any) => void;
}) => {
  return (
    <Group top={node.y} left={node.x} key={node.id}>
      <rect
        onClick={onClick}
        height={node.height}
        width={node.width}
        y={-node.height / 2}
        x={-node.width / 2}
        stroke="#111111"
        strokeWidth={3}
        fill="#EEEEE1"
      />
      <text
        onClick={onClick}
        dy="1.2em"
        fontSize={16}
        fontFamily="monospace"
        textAnchor="middle"
        fill="#111111"
      >
        {node.label}
      </text>
      {hasDatabase && (
        <Group top={-node.height / 2 + 20} left={node.width / 2 - 20}>
          <Circle r={15} fill="#111111" />
          <DatabaseSVG x={-14} y={-400} width={28} />
        </Group>
      )}
      {isOutsideFacing && (
        <Group top={-node.height / 2 + 20} left={-node.width / 2 + 20}>
          <Circle r={15} fill="#111111" />
          <OutsideFacingApiSVG x={-14} y={-400} width={28} />
        </Group>
      )}
    </Group>
  );
};

export const TopicSVG = ({
  node,
  onClick,
}: {
  node: PositionedNode;
  onClick: (event: any) => void;
}) => {
  return (
    <Group top={node.y} left={node.x} key={node.id}>
      <rect
        onClick={onClick}
        height={node.height}
        width={node.width}
        y={-node.height / 2}
        x={-node.width / 2}
        stroke="#111111"
        strokeWidth={3}
        fill="#111111"
      />
      <text
        onClick={onClick}
        dy="0.3em"
        fontSize={16}
        fontFamily="monospace"
        textAnchor="middle"
        fill="#EEEEE1"
      >
        {node.label}
      </text>
    </Group>
  );
};

export const EdgeSVG = ({ edge }: { edge: PositionedEdge }) => {
  const shouldAnimate = edge.type === "publish" || edge.type === "subscription";
  return (
    <LinePath
      curve={curveCatmullRom}
      data={edge.points}
      className={shouldAnimate ? "animate-edge" : ""}
      x={(d) => d.x}
      y={(d) => d.y}
      stroke={"#111111"}
      strokeWidth={2}
      markerEnd="url(#encore-arrow)"
    />
  );
};

export const DatabaseSVG = (props: SVGProps<SVGSVGElement>) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    x={0}
    y={0}
    viewBox="0 0 300 300"
    xmlSpace="preserve"
    {...props}
  >
    <style>{".st0{fill:#EEEEE1;"}</style>
    <path
      className="st0"
      d="M150 177.6c-34.6 0-63.5-10.8-76.9-26.6v20.4c1.7 20.6 35.4 37 76.9 37 42.7 0 77.2-17.3 77.2-38.6v-19.1c-13.2 15.9-42.4 26.9-77.2 26.9z"
    />
    <path
      className="st0"
      d="M150 216.9c-34.6 0-63.5-10.8-76.9-26.6v20.4c1.7 20.6 35.4 37 76.9 37 42.7 0 77.2-17.3 77.2-38.6V190c-13.2 15.9-42.4 26.9-77.2 26.9zM150 138.3c-34.6 0-63.5-10.8-76.9-26.6v20.4c1.7 20.6 35.4 37 76.9 37 42.7 0 77.2-17.3 77.2-38.6v-19.1c-13.2 15.9-42.4 26.9-77.2 26.9z"
    />
    <path
      className="st0"
      d="M150 129.6c42.7 0 77.2-17.3 77.2-38.6h-.3c0-21.3-34.6-38.6-77.2-38.6-41.5 0-75.2 16.4-76.9 37V91c0 21.3 34.6 38.6 77.2 38.6z"
    />
  </svg>
);

export const OutsideFacingApiSVG = (props: SVGProps<SVGSVGElement>) => (
  <svg
    id="Layer_1"
    xmlns="http://www.w3.org/2000/svg"
    x={0}
    y={0}
    viewBox="0 0 300 300"
    xmlSpace="preserve"
    {...props}
  >
    <style>{".st0{fill:#eeeee1}"}</style>
    <Group top={290} left={300}>
      <Group transform="rotate(180)">
        <path
          className="st0"
          d="M204.2 159.5c-30.4-17-47.1-39.2-47.1-62.4 0-3.9-3.2-7.1-7.1-7.1-3.9 0-7.1 3.2-7.1 7.1 0 23.2-16.7 45.4-47.1 62.4-3.4 1.9-4.6 6.2-2.7 9.7 1.9 3.4 6.3 4.6 9.7 2.7 17.5-9.8 30.9-21.3 40.1-34V229c0 3.9 3.2 7.1 7.1 7.1 3.9 0 7.1-3.2 7.1-7.1v-91.1c9.2 12.6 22.7 24.2 40.2 34 1.1.6 2.3.9 3.5.9 2.5 0 4.9-1.3 6.2-3.6 1.8-3.5.6-7.8-2.8-9.7zM230.6 63.9H69.4c-3.9 0-7.1 3.2-7.1 7.1s3.2 7.1 7.1 7.1h161.2c3.9 0 7.1-3.2 7.1-7.1s-3.2-7.1-7.1-7.1z"
        />
      </Group>
    </Group>
  </svg>
);

export const EncoreArrowHeadSVG = (props: SVGProps<SVGMarkerElement>) => (
  <marker
    markerUnits="userSpaceOnUse"
    markerWidth={20}
    markerHeight={20}
    refX={13}
    refY={8}
    orient="auto"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    {...props}
  >
    <path
      d="M2.344 14c2.465-3.708 5.874-6 9.636-6C8.218 8 4.81 5.708 2.344 2"
      fill="none"
      stroke="#111111"
      strokeWidth={3}
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </marker>
);
