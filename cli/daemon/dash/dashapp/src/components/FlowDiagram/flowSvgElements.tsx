import * as React from "react";
import { PropsWithChildren, SVGProps, useEffect, useState } from "react";
import { Group } from "@visx/group";
import { LinePath } from "@visx/shape";
import { Coordinates, EdgeData, PositionedEdge, PositionedNode, ServiceNode } from "./flow-utils";

const OFF_WHITE_COLOR = "#EEEEE1";
const SOFT_BLACK_COLOR = "#111111";
const getActiveClass = (isActive: boolean) => {
  return isActive ? "opacity-100" : "opacity-10";
};

export const ServiceSVG = ({
  node,
  endpoints,
  isActive,
  onClick,
  onMouseEnter,
  onMouseLeave,
}: {
  node: PositionedNode<ServiceNode>;
  isActive: boolean;
  endpoints: { public: number; auth: number; private: number };
  onClick: (event: any) => void;
  onMouseEnter: () => void;
  onMouseLeave: () => void;
}) => {
  const { public: publicEndpoints, auth: authEndpoints, private: privateEndpoints } = endpoints;
  return (
    <Group
      top={node.y}
      left={node.x}
      key={node.id}
      data-testid={`node-${node.id}`}
      className={`node group ${getActiveClass(isActive)}`}
      // onClick={onClick}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
    >
      {/* rect is "shadow" which is visible while hovering */}
      <rect width={node.width} height={node.height} fill={SOFT_BLACK_COLOR} />

      <foreignObject
        className="transform duration-100 ease-in-out group-hover:-translate-x-1 group-hover:-translate-y-1"
        width={node.width}
        height={node.height}
      >
        <div
          className="flex h-full w-full flex-col justify-between border-2 px-2 py-2"
          style={{ background: OFF_WHITE_COLOR, borderColor: SOFT_BLACK_COLOR }}
        >
          <p className="font-mono font-semibold">{node.label}</p>
          <ServiceInfoRow
            data-testid="service-endpoints"
            icon={<EndpointIconSVG />}
            text={
              <p className="flex flex-1 justify-between">
                <span>
                  <b>{publicEndpoints}</b> public
                </span>
                <span>
                  <b>{authEndpoints}</b> auth
                </span>
                <span>
                  <b>{privateEndpoints}</b> private
                </span>
              </p>
            }
          />

          {node.has_database && (
            <ServiceInfoRow icon={<DatabaseIconSVG />} text={<span>Database</span>} />
          )}

          {node.cron_jobs && !!node.cron_jobs.length && (
            <ServiceInfoRow
              data-testid="service-cron-jobs"
              icon={<CronJobsIconSVG />}
              text={
                node.cron_jobs.length === 1 ? (
                  <span className="overflow-hidden overflow-ellipsis whitespace-nowrap">
                    {node.cron_jobs[0].title}
                  </span>
                ) : (
                  <span>
                    <b>{node.cron_jobs.length}</b> cron jobs
                  </span>
                )
              }
            />
          )}
        </div>
      </foreignObject>
    </Group>
  );
};

const ServiceInfoRow = (
  props: PropsWithChildren<
    { icon: JSX.Element; text?: JSX.Element } & React.HTMLAttributes<HTMLDivElement>
  >
) => {
  const { icon, text, ...attr } = props;
  return (
    <div className="flex w-full flex-row pt-1" {...attr}>
      <div className="pr-1">{props.icon}</div>
      <div className="flex w-[calc(100%-1.2rem)] items-center text-xs">{props.text}</div>
    </div>
  );
};

export const TopicSVG = ({
  node,
  isActive,
  onClick,
  onMouseEnter,
  onMouseLeave,
}: {
  node: PositionedNode;
  isActive: boolean;
  onClick: (event: any) => void;
  onMouseEnter: () => void;
  onMouseLeave: () => void;
}) => {
  return (
    <Group
      top={node.y}
      left={node.x}
      key={node.id}
      className={`node ${getActiveClass(isActive)}`}
      // onClick={onClick}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
    >
      <foreignObject width={node.width} height={node.height} className="topic">
        <div
          className="flex h-full w-full items-center justify-center px-2"
          style={{ background: SOFT_BLACK_COLOR }}
        >
          {/*<PubSubIconSVG />*/}
          <p className="pl-1 font-mono" style={{ color: OFF_WHITE_COLOR }}>
            {node.label}
          </p>
        </div>
      </foreignObject>
    </Group>
  );
};

export const EdgeSVG = ({
  edge,
  activeNodeId,
  isInitialRender,
}: {
  edge: PositionedEdge;
  activeNodeId: string | null;
  isInitialRender: boolean;
}) => {
  // we want to add edges directly if it's the initial render
  const [points, setPoints] = useState<Coordinates[]>(isInitialRender ? edge.points : []);
  useEffect(() => {
    if (JSON.stringify(edge.points) !== JSON.stringify(points)) {
      setPoints([]);
      // give some time for nodes to animate before adding back edge
      setTimeout(() => {
        setPoints(edge.points);
      }, 1000);
    }
  }, [edge.points]);
  const shouldAnimate = edge.type === "publish" || edge.type === "subscription";
  const isActive = activeNodeId === null || activeNodeId === edge.source;
  return (
    <LinePath
      // curve={curveCardinal.tension(0.8)}
      data={points}
      className={`edge pointer-events-none ${getActiveClass(isActive)} ${
        shouldAnimate && "animate-message"
      }`}
      x={(d) => d.x}
      y={(d) => d.y}
      stroke={SOFT_BLACK_COLOR}
      strokeWidth={2}
      markerEnd="url(#encore-arrow)"
    />
  );
};

export const EdgeLabelSVG = ({
  edge,
  activeNodeId,
}: {
  edge: PositionedEdge;
  activeNodeId: string | null;
}) => {
  const isArrowPointingUp = (() => {
    const startY = edge.points[0].y;
    const endY = edge.points[edge.points.length - 1].y;
    return startY > endY;
  })();
  const callTypeTextMap: { [key in EdgeData["type"]]: string } = {
    publish: "pub",
    subscription: "sub",
    rpc: "RPCs",
    database: "Uses db",
  };
  const getText = (data: PositionedEdge) => {
    if (edge.type === "database") return callTypeTextMap[edge.type];
    return `${edge.label!.text} ${callTypeTextMap[edge.type]}`;
  };

  if (!edge.label) return null;

  return (
    <foreignObject
      width={80}
      height={25}
      x={edge.label.x - 60}
      y={edge.label.y - (isArrowPointingUp ? 5 : 20)}
      className={`label pointer-events-none ${
        activeNodeId === edge.source ? "opacity-100" : "opacity-0"
      }`}
    >
      <div className="flex h-full items-center justify-center">
        <p
          className="border-gray-700 inline-block rounded border px-1 text-xs"
          style={{ background: SOFT_BLACK_COLOR, color: OFF_WHITE_COLOR }}
        >
          {getText(edge)}
        </p>
      </div>
    </foreignObject>
  );
};

export const DatabaseIconSVG = () => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 20 20"
    fill="currentColor"
    className="h-5 w-5"
  >
    <path
      fillRule="evenodd"
      d="M10 1c3.866 0 7 1.79 7 4s-3.134 4-7 4-7-1.79-7-4 3.134-4 7-4zm5.694 8.13c.464-.264.91-.583 1.306-.952V10c0 2.21-3.134 4-7 4s-7-1.79-7-4V8.178c.396.37.842.688 1.306.953C5.838 10.006 7.854 10.5 10 10.5s4.162-.494 5.694-1.37zM3 13.179V15c0 2.21 3.134 4 7 4s7-1.79 7-4v-1.822c-.396.37-.842.688-1.306.953-1.532.875-3.548 1.369-5.694 1.369s-4.162-.494-5.694-1.37A7.009 7.009 0 013 13.179z"
      clipRule="evenodd"
    />
  </svg>
);

export const CronJobsIconSVG = () => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    fill="none"
    strokeLinecap="round"
    strokeLinejoin="round"
    strokeWidth="2"
    viewBox="0 0 24 24"
    stroke="currentColor"
    className="h-5 w-5"
  >
    <path d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>
);

export const EndpointIconSVG = () => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 20 20"
    fill="currentColor"
    className="h-5 w-5"
  >
    <path
      fillRule="evenodd"
      d="M3 10a.75.75 0 01.75-.75h10.638L10.23 5.29a.75.75 0 111.04-1.08l5.5 5.25a.75.75 0 010 1.08l-5.5 5.25a.75.75 0 11-1.04-1.08l4.158-3.96H3.75A.75.75 0 013 10z"
      clipRule="evenodd"
    />
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
      stroke={SOFT_BLACK_COLOR}
      strokeWidth={3}
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </marker>
);
