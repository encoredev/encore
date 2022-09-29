import React from "react";

export type Icon =
  | ((cls?: string, title?: string) => JSX.Element)
  | ((props: React.ComponentProps<"svg">) => JSX.Element);

// eslint-disable-next-line @typescript-eslint/no-namespace
export namespace icons {
  export const arrowRight: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 35.7 16">
      {renderTitle(title)}
      <path
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M24.2,14c2.5-3.7,5.9-6,9.6-6c-3.8,0-7.2-2.3-9.6-6"
      />
      <path strokeWidth="3" strokeLinecap="round" d="M2,8h31" />
    </svg>
  );

  export const sparkle: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
    >
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z"
      ></path>
    </svg>
  );

  export const scrollToBottom: Icon = (cls, title) => (
    <svg
      className={cls}
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {renderTitle(title)}
      <path d="M17.9,12.2L12,18.1 M12,18.1l-5.9-5.9 M12,18.1V3" />
      <path d="M2.8,21.2h18.4" />
    </svg>
  );

  export const book: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 16 16" fill="currentColor">
      {renderTitle(title)}
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M3.09963 2.39149C3.26933 1.58074 3.9842 1 4.81252 1H12.0002C12.2228 1 12.434 1.09897 12.5765 1.27011C12.719 1.44125 12.7781 1.66686 12.7377 1.88587L10.9877 11.3859C10.9134 11.7895 10.5288 12.0577 10.1255 11.9896C10.0847 11.9964 10.0428 12 10 12H3.25C2.83579 12 2.5 12.3358 2.5 12.75C2.5 13.1642 2.83579 13.5 3.25 13.5H10.595C11.1952 13.5 11.7106 13.0735 11.8229 12.4839L13.5132 3.60968C13.5908 3.20278 13.9834 2.93576 14.3903 3.01326C14.7972 3.09077 15.0643 3.48345 14.9868 3.89035L13.2964 12.7646C13.0494 14.0616 11.9154 15 10.595 15H3.25C2.00736 15 1 13.9927 1 12.75C1 12.6966 1.00186 12.6437 1.00552 12.5912C0.995775 12.5117 0.998723 12.4292 1.01606 12.3464L3.09963 2.39149ZM5.35954 3C5.14433 3 4.95326 3.13772 4.8852 3.34189L4.71853 3.84189C4.61061 4.16565 4.85159 4.5 5.19287 4.5H9.13878C9.354 4.5 9.54507 4.36228 9.61312 4.15811L9.77979 3.65811C9.88771 3.33435 9.64673 3 9.30545 3H5.35954Z"
      />
    </svg>
  );

  export const commit: Icon = (cls, title) => (
    <svg className={cls} fill="currentColor" viewBox="0 0 896 1024">
      {renderTitle(title)}
      <path d="M694.875 448C666.375 337.781 567.125 256 448 256c-119.094 0-218.375 81.781-246.906 192H0v128h201.094C229.625 686.25 328.906 768 448 768c119.125 0 218.375-81.75 246.875-192H896V448H694.875zM448 640c-70.656 0-128-57.375-128-128 0-70.656 57.344-128 128-128 70.625 0 128 57.344 128 128C576 582.625 518.625 640 448 640z" />
    </svg>
  );

  export const lightningBolt: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      {renderTitle(title)}
      <path d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  );

  export const lightBulb: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
    </svg>
  );

  export const code: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      {renderTitle(title)}
      <path d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
    </svg>
  );

  export const cog: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      {renderTitle(title)}
      <path d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  );

  export const cube: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"
      />
    </svg>
  );

  export const cloud: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      stroke="currentColor"
      viewBox="0 0 28 22"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M6.34999 21.2C4.82768 21.198 3.36404 20.6126 2.26025 19.5642C1.15646 18.5159 0.496475 17.0843 0.41611 15.5641C0.335745 14.0439 0.84111 12.5508 1.82822 11.3919C2.81532 10.233 4.20908 9.4965 5.72269 9.334C5.27701 7.58621 5.54388 5.73296 6.46461 4.18194C7.38534 2.63092 8.8845 1.50918 10.6323 1.0635C12.3801 0.617818 14.2333 0.884696 15.7844 1.80542C17.3354 2.72615 18.4571 4.22531 18.9028 5.9731C19.9422 5.82837 21.0004 5.89906 22.0113 6.18078C23.0223 6.4625 23.9645 6.94923 24.7793 7.61068C25.5941 8.27213 26.2641 9.09416 26.7476 10.0256C27.2311 10.9571 27.5178 11.9781 27.5898 13.0251C27.6618 14.0721 27.5176 15.1228 27.1661 16.1117C26.8147 17.1006 26.2636 18.0066 25.547 18.7734C24.8304 19.5401 23.9637 20.1512 23.0008 20.5687C22.038 20.9862 20.9995 21.2011 19.95 21.2H6.34999Z"
        fill="#111111"
      />
    </svg>
  );

  export const cloudDownload: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10" />
    </svg>
  );

  export const cloudUpload: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M8 17a5 5 0 01-.916-9.916 5.002 5.002 0 019.832 0A5.002 5.002 0 0116 17m-7-5l3-3m0 0l3 3m-3-3v12" />
    </svg>
  );

  export const document: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      {renderTitle(title)}
      <path d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
    </svg>
  );

  export const documentReport: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      {renderTitle(title)}
      <path d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
    </svg>
  );

  export const documentDuplicate: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      {renderTitle(title)}
      <path d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2" />
    </svg>
  );

  export const solidDocument: Icon = (cls, title) => (
    <svg className={cls} fill="currentColor" viewBox="0 0 24 24 ">
      {renderTitle(title)}
      <path
        fillRule="evenodd"
        d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z"
        clipRule="evenodd"
      />
    </svg>
  );

  export const pencil: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      {renderTitle(title)}
      <path d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
    </svg>
  );

  export const merge: Icon = (cls, title) => (
    <svg className={cls} fill="currentColor" viewBox="0 0 768 1024">
      {renderTitle(title)}
      <path d="M640 448c-47.625 0-88.625 26.312-110.625 64.906C523.625 512.5 518 512 512 512c-131.062 0-255.438-99.844-300.812-223.438C238.469 265.09400000000005 256 230.71900000000005 256 192c0-70.656-57.344-128-128-128S0 121.34400000000005 0 192c0 47.219 25.844 88.062 64 110.281V721.75C25.844 743.938 0 784.75 0 832c0 70.625 57.344 128 128 128s128-57.375 128-128c0-47.25-25.844-88.062-64-110.25V491.469C276.156 580.5 392.375 640 512 640c6.375 0 11.625-0.438 17.375-0.625C551.5 677.812 592.5 704 640 704c70.625 0 128-57.375 128-128C768 505.344 710.625 448 640 448zM128 896c-35.312 0-64-28.625-64-64 0-35.312 28.688-64 64-64 35.406 0 64 28.688 64 64C192 867.375 163.406 896 128 896zM128 256c-35.312 0-64-28.594-64-64s28.688-64 64-64c35.406 0 64 28.594 64 64S163.406 256 128 256zM640 640c-35.312 0-64-28.625-64-64 0-35.406 28.688-64 64-64 35.375 0 64 28.594 64 64C704 611.375 675.375 640 640 640z" />
    </svg>
  );

  export const calendar: Icon = (cls, title) => (
    <svg className={cls} fill="currentColor" viewBox="0 0 20 20">
      {renderTitle(title)}
      <path
        fillRule="evenodd"
        d="M6 2a1 1 0 00-1 1v1H4a2 2 0 00-2 2v10a2 2 0 002 2h12a2 2 0 002-2V6a2 2 0 00-2-2h-1V3a1 1 0 10-2 0v1H7V3a1 1 0 00-1-1zm0 5a1 1 0 000 2h8a1 1 0 100-2H6z"
        clipRule="evenodd"
      />
    </svg>
  );

  export const search: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      {renderTitle(title)}
      <path d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
    </svg>
  );

  export const chevronUp: Icon = (cls, title) => (
    <svg className={cls} fill="currentColor" viewBox="0 0 12 12">
      {renderTitle(title)}
      <polygon transform="rotate(180 6 6)" points="6,9.7 0,3.7 1.4,2.3 6,6.9 10.6,2.3 12,3.7 	" />
    </svg>
  );

  export const chevronLeft: Icon = (cls, title) => (
    <svg className={cls} fill="currentColor" viewBox="0 0 12 12">
      {renderTitle(title)}
      <polygon transform="rotate(90 6 6)" points="6,9.7 0,3.7 1.4,2.3 6,6.9 10.6,2.3 12,3.7 	" />
    </svg>
  );

  export const chevronDown: Icon = (cls, title) => (
    <svg className={cls} fill="currentColor" viewBox="0 0 12 12">
      {renderTitle(title)}
      <polygon points="6,9.7 0,3.7 1.4,2.3 6,6.9 10.6,2.3 12,3.7 	" />
    </svg>
  );

  export const chevronRight: Icon = (cls, title) => (
    <svg className={cls} fill="currentColor" viewBox="0 0 12 12">
      {renderTitle(title)}
      <polygon transform="rotate(-90 6 6)" points="6,9.7 0,3.7 1.4,2.3 6,6.9 10.6,2.3 12,3.7 	" />
    </svg>
  );

  export const x: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      {renderTitle(title)}
      <path d="M6 18L18 6M6 6l12 12" />
    </svg>
  );

  export const academicCap: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path d="M12 14l9-5-9-5-9 5 9 5z" />
      <path d="M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z" />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M12 14l9-5-9-5-9 5 9 5zm0 0l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14zm-4 6v-7.5l4-2.222"
      />
    </svg>
  );

  export const chatAlt: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z"
      />
    </svg>
  );

  export const circle: Icon = (cls, title) => (
    <svg
      className={cls}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {renderTitle(title)}
      <circle cx="12" cy="12" r="10" />
    </svg>
  );

  export const check: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M5 13l4 4L19 7" />
    </svg>
  );

  export const checkSquare: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      viewBox="0 0 24 24"
    >
      {renderTitle(title)}
      <polyline points="9 11 12 14 22 4" />
      <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
    </svg>
  );

  export const checkCircle: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );

  export const checkCircle2: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
      <polyline points="22 4 12 14.01 9 11.01" />
    </svg>
  );

  export const xCircle: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );

  export const exclamation: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
    </svg>
  );

  export const exclamationCircle: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );

  export const playCircle: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
    </svg>
  );

  export const questionMarkCircle: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );

  export const dotsCircle: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M8 12h.01M12 12h.01M16 12h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );

  export const plusCircle: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M12 9v3m0 0v3m0-3h3m-3 0H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );

  export const minusCircle: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M15 12H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );

  export const plus: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M12 6v6m0 0v6m0-6h6m-6 0H6"
      />
    </svg>
  );

  export const minus: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 12H4" />
    </svg>
  );

  export const menu: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M4 6h16M4 12h16M4 18h16"
      />
    </svg>
  );

  export const stackTrace: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M4 6h16M4 12h8m-8 6h12"
      />
    </svg>
  );

  export const refresh: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
    </svg>
  );

  export const sidebar: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M21,5v14c0,1.1-0.9,2-2,2H5c-1.1,0-2-0.9-2-2V5c0-1.1,0.9-2,2-2h14C20.1,3,21,3.9,21,5z" />
      <line x1="15" y1="20" x2="15" y2="4" />
    </svg>
  );

  export const sidebarOpen: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M21,5v14c0,1.1-0.9,2-2,2H5c-1.1,0-2-0.9-2-2V5c0-1.1,0.9-2,2-2h14C20.1,3,21,3.9,21,5z" />
      <line x1="15" y1="20" x2="15" y2="4" />
      <path
        stroke="none"
        fill="currentColor"
        d="M19,3h-4v18h4c1.1,0,2-0.9,2-2V5C21,3.9,20.1,3,19,3z"
      />
    </svg>
  );

  export const menuAlt2: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M4 6h16M4 12h16M4 18h7"
      />
    </svg>
  );

  export const chip: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 20 20" fill="currentColor">
      {renderTitle(title)}
      <path d="M13 7H7v6h6V7z" />
      <path
        fillRule="evenodd"
        d="M7 2a1 1 0 012 0v1h2V2a1 1 0 112 0v1h2a2 2 0 012 2v2h1a1 1 0 110 2h-1v2h1a1 1 0 110 2h-1v2a2 2 0 01-2 2h-2v1a1 1 0 11-2 0v-1H9v1a1 1 0 11-2 0v-1H5a2 2 0 01-2-2v-2H2a1 1 0 110-2h1V9H2a1 1 0 010-2h1V5a2 2 0 012-2h2V2zM5 5h10v10H5V5z"
        clipRule="evenodd"
      />
    </svg>
  );

  export const clock: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );

  export const fire: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 20 20" fill="currentColor">
      {renderTitle(title)}
      <path
        fillRule="evenodd"
        d="M12.395 2.553a1 1 0 00-1.45-.385c-.345.23-.614.558-.822.88-.214.33-.403.713-.57 1.116-.334.804-.614 1.768-.84 2.734a31.365 31.365 0 00-.613 3.58 2.64 2.64 0 01-.945-1.067c-.328-.68-.398-1.534-.398-2.654A1 1 0 005.05 6.05 6.981 6.981 0 003 11a7 7 0 1011.95-4.95c-.592-.591-.98-.985-1.348-1.467-.363-.476-.724-1.063-1.207-2.03zM12.12 15.12A3 3 0 017 13s.879.5 2.5.5c0-1 .5-4 1.25-4.5.5 1 .786 1.293 1.371 1.879A2.99 2.99 0 0113 13a2.99 2.99 0 01-.879 2.121z"
        clipRule="evenodd"
      />
    </svg>
  );

  export const puzzle: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M11 4a2 2 0 114 0v1a1 1 0 001 1h3a1 1 0 011 1v3a1 1 0 01-1 1h-1a2 2 0 100 4h1a1 1 0 011 1v3a1 1 0 01-1 1h-3a1 1 0 01-1-1v-1a2 2 0 10-4 0v1a1 1 0 01-1 1H7a1 1 0 01-1-1v-3a1 1 0 00-1-1H4a2 2 0 110-4h1a1 1 0 001-1V7a1 1 0 011-1h3a1 1 0 001-1V4z" />
    </svg>
  );

  export const globe: Icon = (cls, title) => (
    <svg className={cls} fill="none" viewBox="2 2 20 20" stroke="currentColor">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );

  export const externalLink: Icon = (cls, title) => (
    <svg className={cls} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
      />
    </svg>
  );

  export const filter: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
    </svg>
  );

  export const slash: Icon = (cls, title) => (
    <svg
      className={cls}
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth="1"
      strokeLinecap="round"
      strokeLinejoin="round"
      fill="none"
      shapeRendering="geometricPrecision"
    >
      {renderTitle(title)}
      <path d="M16.88 3.549L7.12 20.451" />
    </svg>
  );

  export const selector: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
    </svg>
  );

  export const trash: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
      />
    </svg>
  );

  export const user: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
    </svg>
  );

  export const userAdd: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z" />
    </svg>
  );

  export const userRemove: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M13 7a4 4 0 11-8 0 4 4 0 018 0zM9 14a6 6 0 00-6 6v1h12v-1a6 6 0 00-6-6zM21 12h-6" />
    </svg>
  );

  export const userCircle: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M5.121 17.804A13.937 13.937 0 0112 16c2.5 0 4.847.655 6.879 1.804M15 10a3 3 0 11-6 0 3 3 0 016 0zm6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );

  export const layers: Icon = (cls, title) => (
    <svg
      className={cls}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {renderTitle(title)}
      <polygon points="12 2 2 7 12 12 22 7 12 2" />
      <polyline points="2 17 12 22 22 17" />
      <polyline points="2 12 12 17 22 12" />
    </svg>
  );

  export const logout: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 20 20" fill="currentColor">
      {renderTitle(title)}
      <path
        fillRule="evenodd"
        d="M3 3a1 1 0 00-1 1v12a1 1 0 102 0V4a1 1 0 00-1-1zm10.293 9.293a1 1 0 001.414 1.414l3-3a1 1 0 000-1.414l-3-3a1 1 0 10-1.414 1.414L14.586 9H7a1 1 0 100 2h7.586l-1.293 1.293z"
        clipRule="evenodd"
      />
    </svg>
  );

  export const key: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
    </svg>
  );

  export const officeBuilding: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
      />
    </svg>
  );

  export const wrench: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <path d="M 10.78 9.805 L 1.5 19.009 L 4.999 22.497 L 14.26 13.209 C 16.438 14.027 19.205 13.797 20.962 12.048 C 22.858 10.158 22.853 6.986 21.75 4.699 L 17.657 8.779 L 15.205 6.336 L 19.298 2.256 C 17.014 1.146 13.811 1.141 11.916 3.03 C 10.133 4.807 9.911 7.605 10.78 9.805 Z" />
    </svg>
  );

  export const database: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 20 20" fill="currentColor">
      {renderTitle(title)}
      <path d="M3 12v3c0 1.657 3.134 3 7 3s7-1.343 7-3v-3c0 1.657-3.134 3-7 3s-7-1.343-7-3z" />
      <path d="M3 7v3c0 1.657 3.134 3 7 3s7-1.343 7-3V7c0 1.657-3.134 3-7 3S3 8.657 3 7z" />
      <path d="M17 5c0 1.657-3.134 3-7 3S3 6.657 3 5s3.134-3 7-3 7 1.343 7 3z" />
    </svg>
  );

  export const server: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 24 24" fill="none" stroke="currentColor">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"
      />
    </svg>
  );

  export const errCircle: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 20 20" fill="currentColor">
      {renderTitle(title)}
      <path
        fillRule="evenodd"
        d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
        clipRule="evenodd"
      />
    </svg>
  );

  export const photograph: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 20 20" fill="currentColor">
      {renderTitle(title)}
      <path
        fillRule="evenodd"
        d="M4 3a2 2 0 00-2 2v10a2 2 0 002 2h12a2 2 0 002-2V5a2 2 0 00-2-2H4zm12 12H4l4-8 3 6 2-4 3 6z"
        clipRule="evenodd"
      />
    </svg>
  );

  export const camera: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
    >
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"
      ></path>
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M15 13a3 3 0 11-6 0 3 3 0 016 0z"
      ></path>
    </svg>
  );

  export const pulse: Icon = (cls, title) => (
    <svg
      className={cls}
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      {renderTitle(title)}
      <polyline points="2 14.308 5.076 14.308 8.154 2 11.231 20.462 14.308 9.692 15.846 14.308 18.924 14.308" />
      <circle cx="20.462" cy="14.308" r="1.538" />
    </svg>
  );

  export const map: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"
      />
    </svg>
  );

  export const trendingUp: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"
      />
    </svg>
  );

  export const inbox: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M8 4H6a2 2 0 00-2 2v12a2 2 0 002 2h12a2 2 0 002-2V6a2 2 0 00-2-2h-2m-4-1v8m0 0l3-3m-3 3L9 8m-5 5h2.586a1 1 0 01.707.293l2.414 2.414a1 1 0 00.707.293h3.172a1 1 0 00.707-.293l2.414-2.414a1 1 0 01.707-.293H20"
      ></path>
    </svg>
  );

  export const arrowsExpand: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
      ></path>
    </svg>
  );

  export const archiveBoxArrowDown: Icon = (cls, title) => (
    <svg className={cls} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m8.25 3v6.75m0 0l-3-3m3 3l3-3M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z"
      />
    </svg>
  );

  export const archiveBoxArrowUp: Icon = (cls, title) => (
    <svg className={cls} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M21.5192 6.82692L20.7981 19.0946C20.7592 19.7558 20.4691 20.3772 19.9871 20.8315C19.5051 21.2858 18.8677 21.5387 18.2054 21.5385H5.79462C5.13226 21.5387 4.49486 21.2858 4.01288 20.8315C3.53089 20.3772 3.24078 19.7558 3.20192 19.0946L2.48077 6.82692M12 18.0769V10.2885M12 10.2885L8.53846 13.75M12 10.2885L15.4615 13.75M2.04808 6.82692H21.9519C22.6685 6.82692 23.25 6.24538 23.25 5.52885V3.79808C23.25 3.08154 22.6685 2.5 21.9519 2.5H2.04808C1.33154 2.5 0.75 3.08154 0.75 3.79808V5.52885C0.75 6.24538 1.33154 6.82692 2.04808 6.82692Z"
      />
    </svg>
  );

  export const shield: Icon = (cls, title) => (
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {renderTitle(title)}
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
      ></path>
    </svg>
  );

  export const githubLogo: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 16 16" fill="currentColor">
      {renderTitle(title)}
      <path
        fillRule="evenodd"
        d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"
      />
    </svg>
  );

  export const azureLogo: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 19.7 15.2" fill="currentColor">
      {renderTitle(title)}
      <path d="M 9.105 14.431 C 11.634 13.984 13.723 13.614 13.747 13.609 L 13.79 13.6 L 11.403 10.76 C 10.09 9.199 9.016 7.915 9.016 7.907 C 9.016 7.893 11.481 1.105 11.495 1.081 C 11.499 1.073 13.177 3.969 15.561 8.102 C 17.793 11.971 19.634 15.161 19.651 15.191 L 19.682 15.245 L 12.095 15.244 L 4.508 15.243 L 9.105 14.431 Z M 0 13.565 C 0 13.561 1.125 11.608 2.5 9.225 L 5 4.893 L 7.913 2.448 C 9.515 1.104 10.83 0.002 10.836 0 C 10.841 -0.002 10.82 0.051 10.789 0.118 C 10.758 0.185 9.334 3.238 7.625 6.903 L 4.518 13.566 L 2.259 13.569 C 1.017 13.571 0 13.569 0 13.565 Z" />
    </svg>
  );

  export const encoreLogo: Icon = (cls, title) => (
    <svg
      className={cls}
      version="1.1"
      x="0px"
      y="0px"
      viewBox="90.6 91 90.9 102.1"
      fill="currentColor"
      aria-labelledby="title"
    >
      {renderTitle(title ?? "Encore")}
      <g>
        <path
          d="M181.4,170.2v22.9H90.6v-69.3c14.4-3.1,28.7-7.1,42.6-12c16.6-5.8,32.7-12.7,48.3-20.8v25.6
		c-13.2,6.4-26.9,12-40.8,16.9c-15.7,5.5-31.8,9.9-48.1,13.3v0.2c30.1-2.8,59.7-7.7,88.9-14.5v23.5c-29.2,6.6-58.9,11.3-88.9,14v0.2
		H181.4z"
        />
      </g>
    </svg>
  );

  export const encoreWordmark: Icon = (cls, title) => (
    <svg
      className={cls}
      version="1.1"
      x="0px"
      y="0px"
      viewBox="90.5 97.4 414.9 89.2"
      fill="currentColor"
      aria-labelledby="title"
    >
      {renderTitle(title ?? "Encore")}
      <g>
        <path
          d="M466.9,118.4c-10.4-0.1-16.8,6.3-15.2,22.1c9.1-3.6,18.4-7.9,27.7-13.1C476.7,119.2,469.7,118.4,466.9,118.4z
		 M470.9,184.8c-26.3-0.2-42.6-17-42.6-44c0-26.7,15.2-43.3,39.7-43.3c20.6,0,33.5,12.9,37.4,37.3c-14.2,9.4-30,17.7-46.6,24.1
		c0,0,4.3,3.1,13.2,3h32.4v22.9H470.9z M219.1,184.8v-49.3c0-11.1-3-14.3-13.3-14.3h-10.9v63.7h-23.1V98.6h36.7
		c22.7,0,33.7,10.4,33.7,31.9v54.3H219.1z M289.4,184.8c-26.4,0-42.8-16.3-42.8-42.9c0-26.8,16.3-43.3,42.6-43.3h10v23.6h-8.4
		c-13,0.2-20.4,7.4-20.4,19.8c0,12,7.4,19,20.4,19.1h8.8v23.6H289.4z M383.5,184.8v-54.3c0-21.5,11-31.9,33.7-31.9h10.7v22.6h-8
		c-10.3,0-13.3,3.2-13.3,14.3v49.3H383.5z M129.1,118.4c-10.4-0.1-16.8,6.3-15.2,22.1c9.1-3.6,18.4-7.9,27.7-13.1
		C138.9,119.2,131.9,118.4,129.1,118.4z M133.1,184.8c-26.3-0.2-42.6-17-42.6-44c0-26.7,15.2-43.3,39.7-43.3
		c20.6,0,33.5,12.9,37.4,37.3c-14.2,9.4-30,17.7-46.6,24.1c0,0,4.3,3.1,13.2,3h32.4v22.9H133.1z M301.4,141.7
		c0,27.7,14.6,44.9,38.8,44.9s38.8-17.2,38.8-44.9c0.1-14.7-3.8-26.1-11.5-33.8c-6.7-6.8-16.4-10.5-27.3-10.5
		c-10.9,0-20.6,3.7-27.3,10.5C305.2,115.7,301.3,127,301.4,141.7z M323.6,142.1c0-13.1,6.7-22,16.6-22c9.9,0,16.6,8.8,16.6,22
		c0,13.1-6.6,21.6-16.6,21.6S323.6,155.2,323.6,142.1z"
        />
      </g>
    </svg>
  );

  export const encoreFullLogo: Icon = (cls, title) => (
    <svg
      className={cls}
      version="1.1"
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 527.7 102.2"
      fill="currentColor"
      aria-labelledby="title"
    >
      {renderTitle(title ?? "Encore")}
      <path
        d="M490.2,35.7c-10.4-0.1-16.8,6.3-15.2,22.1c9.1-3.6,18.4-7.9,27.7-13.1C500,36.6,493,35.7,490.2,35.7z
          M494.2,102.2c-26.3-0.2-42.6-17-42.6-44c0-26.7,15.2-43.3,39.7-43.3c20.6,0,33.5,12.9,37.4,37.3c-14.2,9.4-30,17.7-46.6,24.1
          c0,0,4.3,3.1,13.2,3h32.4v22.9H494.2z M242.4,102.2V52.8c0-11.1-3-14.3-13.3-14.3h-11v63.7H195V15.9h36.7
          c22.7,0,33.7,10.4,33.7,31.9v54.4H242.4z M312.7,102.2c-26.4,0-42.8-16.3-42.8-42.9c0-26.8,16.3-43.3,42.6-43.3h10v23.6h-8.4
          c-13,0.2-20.4,7.4-20.4,19.8c0,12,7.4,19,20.4,19.1h8.8v23.7H312.7z M406.8,102.2V47.8c0-21.5,11-31.9,33.7-31.9h10.7v22.6h-8
          c-10.3,0-13.3,3.2-13.3,14.3v49.4H406.8z M152.4,35.7c-10.4-0.1-16.8,6.3-15.2,22.1c9.1-3.6,18.4-7.9,27.7-13.1
          C162.2,36.6,155.2,35.7,152.4,35.7z M156.4,102.2c-26.3-0.2-42.6-17-42.6-44c0-26.7,15.2-43.3,39.7-43.3
          c20.6,0,33.5,12.9,37.4,37.3c-14.2,9.4-30,17.7-46.6,24.1c0,0,4.3,3.1,13.2,3h32.4v22.9H156.4z M90.9,79.2v23H0V32.8
          c14.4-3.1,28.7-7.1,42.6-12C59.2,15,75.3,8.1,90.9,0v25.6C77.7,32,64,37.6,50.1,42.5C34.4,48,18.3,52.4,2,55.8V56
          c30.1-2.8,59.7-7.7,88.9-14.5V65C61.7,71.6,32,76.3,2,79v0.2H90.9z M324.7,59c0,27.7,14.6,44.9,38.8,44.9s38.8-17.2,38.8-44.9
          c0.1-14.7-3.8-26.1-11.5-33.8c-6.7-6.8-16.4-10.5-27.3-10.5s-20.6,3.7-27.3,10.5C328.5,33,324.6,44.4,324.7,59z M346.9,59.4
          c0-13.1,6.7-22,16.6-22s16.6,8.8,16.6,22c0,13.1-6.6,21.6-16.6,21.6C353.5,81,346.9,72.5,346.9,59.4z"
      />
    </svg>
  );

  export const awsLogo = (
    cls: string | undefined,
    title: string | undefined,
    boxColor = "currentColor"
  ) => (
    <svg className={cls} viewBox="0 0 580 217.5">
      <g fill={boxColor}>
        <path d="M 161.209 75 L 127.5 63.75 L 127.5 15 L 161.25 25 L 161.209 75" />
        <path d="M 170 25 L 203.75 15 L 203.633 63.716 L 170 75 L 170 25" />
        <path d="M 196.875 9.721 L 165.42 0 L 134.375 9.721 L 165.465 18.055 L 196.875 9.721" />
        <path d="M 246.209 75 L 212.5 63.75 L 212.5 15 L 246.25 25 L 246.209 75" />
        <path d="M 255 25 L 288.75 15 L 288.633 63.716 L 255 75 L 255 25" />
        <path d="M 281.875 9.721 L 250.42 0 L 219.375 9.721 L 250.465 18.055 L 281.875 9.721" />
        <path d="M 33.75 96.25 L 0 86.25 L 0 135 L 33.709 146.25 L 33.75 96.25" />
        <path d="M 42.5 96.25 L 76.25 86.25 L 76.133 134.968 L 42.5 146.25 L 42.5 96.25" />
        <path d="M 69.375 80.971 L 37.92 71.25 L 6.875 80.971 L 37.965 89.305 L 69.375 80.971" />
        <path d="M 118.709 146.25 L 85 135 L 85 86.25 L 118.75 96.25 L 118.709 146.25" />
        <path d="M 127.5 96.25 L 161.25 86.25 L 161.133 134.968 L 127.5 146.25 L 127.5 96.25" />
        <path d="M 154.375 80.971 L 122.92 71.25 L 91.875 80.971 L 122.965 89.305 L 154.375 80.971" />
        <path d="M 76.209 217.5 L 42.5 206.25 L 42.5 157.5 L 76.25 167.5 L 76.209 217.5" />
        <path d="M 85 167.5 L 118.75 157.5 L 118.633 206.218 L 85 217.5 L 85 167.5" />
        <path d="M 111.875 152.221 L 80.42 142.5 L 49.375 152.221 L 80.465 160.555 L 111.875 152.221" />
      </g>
      <g fill="currentColor">
        <path d="M 225.487 174.406 L 223.008 174.415 C 222.097 174.415 220.938 175.03 220.652 176.026 L 213.129 202.533 L 205.688 176.084 C 205.464 175.255 204.683 174.415 203.62 174.415 L 200.684 174.415 C 199.626 174.415 198.855 175.266 198.641 176.101 L 191.719 202.275 L 184.566 176.02 C 184.284 175.06 183.095 174.415 182.162 174.415 L 178.594 174.409 C 177.831 174.409 177.123 174.736 176.743 175.264 C 176.485 175.621 176.414 176.026 176.532 176.379 L 187.582 215.499 C 187.826 216.321 188.539 217.15 189.581 217.15 L 193.13 217.15 C 194.089 217.15 194.929 216.451 195.171 215.46 L 201.972 189.401 L 209.374 215.49 C 209.596 216.289 210.313 217.15 211.36 217.15 L 214.851 217.15 C 215.823 217.15 216.64 216.475 216.88 215.486 L 227.516 176.395 C 227.641 176.019 227.569 175.6 227.315 175.248 C 226.941 174.729 226.241 174.406 225.487 174.406" />
        <path d="M 256.173 191.015 L 237.425 191.015 C 237.856 185.754 241.273 180.299 247.06 180.089 C 253.219 180.29 256.036 185.62 256.173 191.015 Z M 247.046 174.126 C 234.383 174.126 228.615 185.323 228.615 195.724 C 228.615 208.705 235.785 217.093 246.881 217.093 C 254.823 217.093 260.876 213.08 263.491 206.07 C 263.631 205.669 263.588 205.234 263.369 204.844 C 263.103 204.37 262.598 204.013 262 203.881 L 258.485 203.209 C 257.541 203.061 256.46 203.62 256.103 204.431 C 254.395 208.326 251.283 210.536 247.376 210.651 C 243.618 210.535 240.166 208.303 238.585 204.966 C 237.3 202.241 237.11 199.555 237.089 196.824 L 262.54 196.815 C 263.074 196.815 263.626 196.58 264.016 196.185 C 264.343 195.858 264.52 195.45 264.516 195.043 C 264.454 184.93 259.833 174.126 247.046 174.126" />
        <path d="M 295.828 195.786 C 295.828 198.181 295.394 210.14 286.898 210.446 C 284.05 210.338 281.173 208.664 279.565 206.189 C 278.381 204.318 277.69 201.73 277.513 198.529 L 277.513 191.493 C 277.651 186.103 281.445 180.188 286.87 179.959 C 295.395 180.295 295.828 193.201 295.828 195.786 Z M 288.23 173.764 L 287.556 173.764 C 283.352 173.764 280.049 175.51 277.513 179.089 L 277.509 162.598 C 277.509 161.671 276.553 160.828 275.496 160.828 L 271.836 160.828 C 270.88 160.828 269.81 161.585 269.808 162.598 L 269.808 214.078 C 269.811 215.013 270.768 215.863 271.823 215.863 L 272.898 215.856 C 273.913 215.856 274.635 215.039 274.879 214.279 L 276.024 210.645 C 278.684 214.423 282.69 216.641 286.977 216.641 L 287.634 216.641 C 298.928 216.641 303.983 205.656 303.983 194.77 C 303.983 189.351 302.646 184.17 300.318 180.555 C 297.626 176.366 292.994 173.764 288.23 173.764" />
        <path d="M 344.059 194.628 C 341.679 192.918 338.818 192.346 335.951 191.774 L 330.455 190.755 C 326.511 190.106 324.229 189.049 324.229 185.569 C 324.229 181.89 328.041 180.484 331.274 180.396 C 335.221 180.496 338.19 182.258 339.631 185.355 C 339.974 186.085 340.733 186.576 341.521 186.576 C 341.656 186.576 341.794 186.561 341.924 186.533 L 345.354 185.775 C 345.914 185.651 346.436 185.267 346.716 184.769 C 346.94 184.373 346.989 183.94 346.855 183.554 C 344.756 177.44 339.427 174.209 331.366 174.209 C 324.083 174.225 316.299 177.464 316.299 186.51 C 316.299 192.693 320.184 196.619 327.854 198.181 L 334.006 199.354 C 337.384 200 340.838 201.15 340.838 204.776 C 340.838 210.145 334.59 210.693 332.701 210.731 C 328.464 210.64 323.666 208.75 322.501 204.748 C 322.268 203.868 321.16 203.245 320.189 203.448 L 316.601 204.188 C 316.061 204.301 315.568 204.653 315.28 205.126 C 315.039 205.521 314.97 205.964 315.081 206.368 C 316.933 213.009 323.344 217.014 332.231 217.081 L 332.429 217.083 C 340.366 217.083 348.847 213.684 348.847 204.149 C 348.847 200.268 347.101 196.798 344.059 194.628" />
        <path d="M 380.344 191.054 L 361.591 191.054 C 362.02 185.788 365.436 180.33 371.224 180.13 C 377.38 180.325 380.201 185.655 380.344 191.054 Z M 371.214 174.163 C 358.548 174.163 352.779 185.359 352.779 195.76 C 352.779 208.74 359.95 217.126 371.048 217.126 C 378.99 217.126 385.044 213.114 387.661 206.105 C 387.8 205.704 387.756 205.268 387.536 204.876 C 387.27 204.403 386.764 204.045 386.168 203.916 L 382.648 203.244 C 381.824 203.11 380.675 203.536 380.27 204.468 C 378.561 208.36 375.448 210.57 371.545 210.69 C 367.783 210.57 364.333 208.338 362.758 205.003 C 361.47 202.284 361.279 199.595 361.26 196.858 L 386.709 196.851 C 387.239 196.851 387.788 196.618 388.177 196.225 C 388.505 195.896 388.683 195.489 388.679 195.079 C 388.62 184.968 384.003 174.163 371.214 174.163" />
        <path d="M 412.711 174.639 C 412.215 174.584 411.737 174.558 411.274 174.558 C 407.002 174.558 403.534 176.88 401.126 181.316 L 401.135 177.431 C 401.131 176.49 400.193 175.664 399.126 175.664 L 395.948 175.664 C 394.905 175.664 393.985 176.494 393.978 177.446 L 393.972 215.188 C 393.972 216.135 394.893 216.966 395.943 216.966 L 399.651 216.965 C 400.616 216.965 401.703 216.208 401.711 215.188 L 401.714 196.16 C 401.714 193.023 402.006 190.659 403.505 187.863 C 405.646 183.879 408.636 181.925 412.646 181.894 C 413.651 181.886 414.534 181.009 414.534 180.015 L 414.534 176.485 C 414.534 175.561 413.732 174.75 412.711 174.639" />
        <path d="M 451.65 173.898 L 448.721 173.899 C 447.804 173.899 446.628 174.558 446.343 175.524 L 436.849 205.363 L 427.154 175.554 C 426.868 174.564 425.689 173.899 424.766 173.899 L 420.584 173.895 C 419.816 173.895 419.074 174.238 418.694 174.766 C 418.439 175.12 418.369 175.521 418.495 175.896 L 431.764 215.258 C 432.019 216.036 432.695 216.946 433.765 216.946 L 438.826 216.946 C 439.755 216.946 440.516 216.314 440.865 215.254 L 453.735 175.909 C 453.864 175.535 453.795 175.133 453.541 174.778 C 453.159 174.244 452.418 173.898 451.65 173.898" />
        <path d="M 461.273 158.768 C 458.575 158.768 456.381 160.968 456.381 163.674 C 456.381 166.38 458.575 168.579 461.273 168.579 C 463.971 168.579 466.168 166.38 466.168 163.674 C 466.168 160.968 463.971 158.768 461.273 158.768" />
        <path d="M 463.286 173.658 L 459.27 173.654 C 458.177 173.654 457.183 174.538 457.183 175.506 L 457.163 215.58 C 457.163 216.065 457.413 216.546 457.845 216.904 C 458.241 217.229 458.748 217.416 459.256 217.416 L 463.301 217.423 L 463.304 217.423 C 464.412 217.413 465.386 216.553 465.386 215.583 L 465.386 175.506 C 465.386 174.521 464.404 173.658 463.286 173.658" />
        <path d="M 500.82 200.833 L 497.466 200.845 C 496.53 200.845 495.745 201.408 495.396 202.385 C 494.171 207.67 491.396 210.409 487.189 210.53 C 478.994 210.286 478.357 198.864 478.357 195.369 C 478.357 188.399 480.771 180.878 487.51 180.679 C 491.578 180.804 494.536 183.72 495.424 188.473 C 495.591 189.443 496.336 190.176 497.354 190.304 L 500.943 190.339 C 502.015 190.223 502.856 189.411 502.849 188.405 C 501.579 179.816 495.628 174.265 487.674 174.265 L 487.399 174.273 L 487.088 174.265 C 475.52 174.265 470.341 185.061 470.341 195.76 C 470.341 205.569 474.708 217.023 487.023 217.023 L 487.612 217.023 C 495.367 217.023 501.221 211.586 502.898 202.773 C 502.935 202.353 502.79 201.933 502.493 201.591 C 502.107 201.149 501.494 200.865 500.82 200.833" />
        <path d="M 533.943 191.054 L 515.188 191.054 C 515.616 185.786 519.034 180.33 524.825 180.13 C 530.983 180.325 533.803 185.655 533.943 191.054 Z M 524.813 174.163 C 512.148 174.163 506.38 185.359 506.38 195.76 C 506.38 208.74 513.551 217.126 524.649 217.126 C 532.588 217.126 538.639 213.116 541.259 206.106 C 541.395 205.703 541.35 205.265 541.131 204.876 C 540.861 204.399 540.37 204.05 539.764 203.916 L 536.249 203.244 C 535.421 203.11 534.276 203.536 533.871 204.468 C 532.158 208.36 529.043 210.57 525.141 210.691 C 521.38 210.57 517.931 208.338 516.356 205.003 C 515.068 202.28 514.876 199.591 514.856 196.858 L 540.308 196.854 C 540.843 196.854 541.395 196.618 541.786 196.221 C 542.111 195.89 542.286 195.483 542.28 195.079 C 542.221 184.968 537.603 174.163 524.813 174.163" />
        <path d="M 574.164 194.628 C 571.78 192.916 568.915 192.345 566.054 191.774 L 560.558 190.755 C 556.618 190.106 554.336 189.049 554.336 185.569 C 554.336 180.824 560.225 180.428 561.374 180.396 C 565.325 180.496 568.294 182.258 569.733 185.356 C 570.078 186.086 570.839 186.576 571.629 186.576 C 571.765 186.576 571.901 186.561 572.034 186.533 L 575.461 185.775 C 576.021 185.651 576.543 185.266 576.823 184.767 C 577.045 184.371 577.094 183.94 576.96 183.554 C 574.864 177.44 569.534 174.209 561.469 174.209 C 554.188 174.225 546.406 177.464 546.406 186.51 C 546.406 192.694 550.29 196.62 557.959 198.181 L 564.114 199.354 C 567.491 200 570.945 201.15 570.945 204.776 C 570.945 210.145 564.698 210.693 562.809 210.731 C 558.88 210.648 553.851 209.025 552.61 204.753 C 552.383 203.868 551.276 203.246 550.288 203.448 L 546.706 204.188 C 546.164 204.303 545.669 204.656 545.381 205.131 C 545.142 205.528 545.074 205.966 545.186 206.368 C 547.033 213.009 553.443 217.014 562.331 217.081 L 562.529 217.083 C 570.468 217.083 578.95 213.684 578.95 204.149 C 578.95 200.265 577.205 196.794 574.164 194.628" />
        <path d="M 410.894 96.503 L 410.894 87.403 C 410.899 86.023 411.946 85.096 413.204 85.098 L 453.972 85.096 C 455.274 85.096 456.324 86.045 456.324 87.393 L 456.324 95.196 C 456.311 96.503 455.208 98.21 453.252 100.923 L 432.136 131.074 C 439.971 130.887 448.266 132.061 455.386 136.064 C 456.995 136.966 457.423 138.305 457.554 139.613 L 457.554 149.323 C 457.554 150.661 456.089 152.209 454.551 151.403 C 442.002 144.828 425.349 144.11 411.47 151.479 C 410.054 152.235 408.569 150.711 408.569 149.373 L 408.569 140.146 C 408.569 138.674 408.599 136.145 410.085 133.893 L 434.554 98.795 L 413.252 98.79 C 411.949 98.79 410.906 97.863 410.894 96.503" />
        <path d="M 262.181 153.318 L 249.775 153.318 C 248.596 153.24 247.653 152.351 247.559 151.218 L 247.568 87.558 C 247.568 86.284 248.64 85.266 249.96 85.266 L 261.511 85.26 C 262.721 85.327 263.689 86.24 263.771 87.398 L 263.771 95.715 L 264.001 95.715 C 267.009 87.674 272.683 83.929 280.323 83.929 C 288.079 83.929 292.943 87.674 296.416 95.715 C 299.429 87.674 306.258 83.929 313.558 83.929 C 318.768 83.929 324.434 86.069 327.908 90.889 C 331.844 96.25 331.033 104.015 331.033 110.85 L 331.025 151.024 C 331.025 152.294 329.954 153.318 328.635 153.318 L 316.248 153.318 C 315 153.24 314.019 152.245 314.019 151.03 L 314.019 117.279 C 314.019 114.601 314.248 107.9 313.67 105.358 C 312.741 101.066 309.961 99.868 306.378 99.868 C 303.364 99.868 300.236 101.878 298.963 105.085 C 297.69 108.305 297.808 113.664 297.808 117.279 L 297.808 151.024 C 297.808 152.294 296.736 153.318 295.415 153.318 L 283.025 153.318 C 281.784 153.24 280.8 152.245 280.8 151.03 L 280.784 117.279 C 280.784 110.179 281.945 99.734 273.148 99.734 C 264.233 99.734 264.58 109.913 264.58 117.279 L 264.573 151.024 C 264.578 152.294 263.505 153.318 262.181 153.318" />
        <path d="M 491.536 96.918 C 482.385 96.918 481.809 109.375 481.809 117.148 C 481.809 124.915 481.694 141.525 491.424 141.525 C 501.035 141.525 501.496 128.129 501.496 119.959 C 501.496 114.601 501.268 108.173 499.646 103.083 C 498.252 98.66 495.476 96.918 491.536 96.918 Z M 491.424 83.929 C 509.831 83.929 519.79 99.734 519.79 119.82 C 519.79 139.249 508.786 154.655 491.424 154.655 C 473.36 154.655 463.516 138.845 463.516 119.154 C 463.516 99.329 473.471 83.929 491.424 83.929" />
        <path d="M 543.665 153.318 L 531.306 153.318 C 530.061 153.24 529.08 152.245 529.08 151.03 L 529.063 87.341 C 529.165 86.181 530.195 85.266 531.44 85.266 L 542.949 85.26 C 544.034 85.32 544.924 86.058 545.151 87.044 L 545.151 96.785 L 545.386 96.785 C 548.855 88.076 553.721 83.929 562.285 83.929 C 567.844 83.929 573.289 85.933 576.76 91.426 C 580 96.515 580 105.085 580 111.249 L 580 151.31 C 579.861 152.439 578.855 153.318 577.63 153.318 L 565.19 153.318 C 564.043 153.243 563.12 152.395 562.981 151.31 L 562.981 116.743 C 562.981 109.778 563.794 99.595 555.223 99.595 C 552.214 99.595 549.439 101.609 548.048 104.686 C 546.309 108.569 546.079 112.454 546.079 116.743 L 546.079 151.024 C 546.058 152.294 544.985 153.318 543.665 153.318" />
        <path
          fillRule="evenodd"
          d="M 390.899 153.156 C 390.076 153.89 388.899 153.939 387.971 153.445 C 383.852 150.021 383.113 148.434 380.861 145.178 C 374.053 152.113 369.231 154.193 360.415 154.193 C 349.969 154.193 341.849 147.748 341.849 134.86 C 341.849 124.794 347.3 117.946 355.074 114.589 C 361.804 111.634 371.204 111.096 378.396 110.288 L 378.396 108.679 C 378.396 105.727 378.628 102.235 376.885 99.686 C 375.381 97.404 372.48 96.466 369.924 96.466 C 365.193 96.466 360.989 98.891 359.954 103.916 C 359.744 105.031 358.924 106.14 357.8 106.194 L 345.781 104.891 C 344.764 104.663 343.638 103.851 343.931 102.303 C 346.694 87.715 359.864 83.308 371.668 83.308 C 377.698 83.308 385.591 84.917 390.349 89.481 C 396.38 95.123 395.799 102.639 395.799 110.829 L 395.799 130.153 C 395.799 135.965 398.215 138.516 400.486 141.649 C 401.273 142.779 401.45 144.118 400.44 144.946 C 397.908 147.069 393.406 150.975 390.933 153.184 L 390.899 153.156 Z M 378.396 122.911 C 378.396 127.745 378.511 131.776 376.074 136.069 C 374.101 139.563 370.966 141.709 367.485 141.709 C 362.732 141.709 359.949 138.085 359.949 132.713 C 359.949 122.151 369.426 120.228 378.396 120.228 L 378.396 122.911"
        />
        <path
          fillRule="evenodd"
          d="M 228.306 153.156 C 227.486 153.89 226.3 153.939 225.373 153.445 C 221.254 150.021 220.516 148.434 218.263 145.178 C 211.457 152.113 206.634 154.193 197.813 154.193 C 187.37 154.193 179.248 147.748 179.248 134.86 C 179.248 124.794 184.704 117.946 192.477 114.589 C 199.209 111.634 208.607 111.096 215.8 110.288 L 215.8 108.679 C 215.8 105.727 216.034 102.235 214.291 99.686 C 212.784 97.404 209.88 96.466 207.331 96.466 C 202.6 96.466 198.391 98.891 197.36 103.916 C 197.145 105.031 196.329 106.14 195.206 106.194 L 183.183 104.891 C 182.168 104.663 181.04 103.851 181.331 102.303 C 184.094 87.715 197.265 83.308 209.068 83.308 C 215.105 83.308 222.993 84.917 227.751 89.481 C 233.785 95.123 233.203 102.639 233.203 110.829 L 233.203 130.153 C 233.203 135.965 235.62 138.516 237.885 141.649 C 238.675 142.779 238.854 144.118 237.846 144.946 C 235.311 147.069 230.807 150.975 228.335 153.184 L 228.306 153.156 Z M 215.8 122.911 C 215.8 127.745 215.914 131.776 213.481 136.069 C 211.51 139.563 208.373 141.709 204.896 141.709 C 200.134 141.709 197.351 138.085 197.351 132.713 C 197.351 122.151 206.826 120.228 215.8 120.228 L 215.8 122.911"
        />
      </g>
    </svg>
  );

  export const gcpLogo: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 18 14.5">
      {renderTitle(title)}
      <path
        fill="#EA4335"
        d="M 11.414 3.993 L 11.963 3.993 L 13.527 2.428 L 13.604 1.764 C 10.692 -0.807 6.248 -0.529 3.678 2.383 C 2.963 3.191 2.446 4.154 2.163 5.195 C 2.337 5.124 2.53 5.112 2.712 5.163 L 5.841 4.646 C 5.841 4.646 6 4.383 6.083 4.399 C 7.474 2.87 9.817 2.692 11.425 3.993 L 11.414 3.993 Z"
      />
      <path
        fill="#4285F4"
        d="M 15.757 5.195 C 15.397 3.871 14.659 2.68 13.632 1.77 L 11.437 3.966 C 12.364 4.723 12.892 5.865 12.869 7.062 L 12.869 7.452 C 13.949 7.452 14.824 8.328 14.824 9.407 C 14.824 10.486 13.949 11.362 12.869 11.362 L 8.96 11.362 L 8.57 11.757 L 8.57 14.102 L 8.96 14.492 L 12.869 14.492 C 15.677 14.514 17.97 12.255 17.992 9.448 C 18.005 7.743 17.165 6.148 15.757 5.195 Z"
      />
      <path
        fill="#34A853"
        d="M 5.046 14.467 L 8.955 14.467 L 8.955 11.338 L 5.046 11.338 C 4.767 11.338 4.492 11.277 4.238 11.162 L 3.69 11.333 L 2.114 12.897 L 1.977 13.446 C 2.86 14.114 3.938 14.472 5.046 14.467 Z"
      />
      <path
        fill="#FBBC05"
        d="M 5.046 4.317 C 2.237 4.333 -0.024 6.623 -0.008 9.431 C 0.002 10.999 0.733 12.475 1.977 13.43 L 4.244 11.163 C 3.261 10.719 2.823 9.56 3.267 8.577 C 3.711 7.594 4.869 7.156 5.853 7.6 C 6.286 7.796 6.633 8.144 6.829 8.577 L 9.096 6.31 C 8.133 5.048 6.633 4.31 5.046 4.317 Z"
      />
    </svg>
  );

  export const gcpLogoMono: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 18 14.5" fill="currentColor">
      {renderTitle(title)}
      <path d="M 11.414 3.993 L 11.963 3.993 L 13.527 2.428 L 13.604 1.764 C 10.692 -0.807 6.248 -0.529 3.678 2.383 C 2.963 3.191 2.446 4.154 2.163 5.195 C 2.337 5.124 2.53 5.112 2.712 5.163 L 5.841 4.646 C 5.841 4.646 6 4.383 6.083 4.399 C 7.474 2.87 9.817 2.692 11.425 3.993 L 11.414 3.993 Z" />
      <path d="M 15.757 5.195 C 15.397 3.871 14.659 2.68 13.632 1.77 L 11.437 3.966 C 12.364 4.723 12.892 5.865 12.869 7.062 L 12.869 7.452 C 13.949 7.452 14.824 8.328 14.824 9.407 C 14.824 10.486 13.949 11.362 12.869 11.362 L 8.96 11.362 L 8.57 11.757 L 8.57 14.102 L 8.96 14.492 L 12.869 14.492 C 15.677 14.514 17.97 12.255 17.992 9.448 C 18.005 7.743 17.165 6.148 15.757 5.195 Z" />
      <path d="M 5.046 14.467 L 8.955 14.467 L 8.955 11.338 L 5.046 11.338 C 4.767 11.338 4.492 11.277 4.238 11.162 L 3.69 11.333 L 2.114 12.897 L 1.977 13.446 C 2.86 14.114 3.938 14.472 5.046 14.467 Z" />
      <path d="M 5.046 4.317 C 2.237 4.333 -0.024 6.623 -0.008 9.431 C 0.002 10.999 0.733 12.475 1.977 13.43 L 4.244 11.163 C 3.261 10.719 2.823 9.56 3.267 8.577 C 3.711 7.594 4.869 7.156 5.853 7.6 C 6.286 7.796 6.633 8.144 6.829 8.577 L 9.096 6.31 C 8.133 5.048 6.633 4.31 5.046 4.317 Z" />
    </svg>
  );

  export const awsLogoMono: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 304 182" fill="currentColor">
      <g>
        <path
          d="M86.4,66.4c0,3.7,0.4,6.7,1.1,8.9c0.8,2.2,1.8,4.6,3.2,7.2c0.5,0.8,0.7,1.6,0.7,2.3c0,1-0.6,2-1.9,3l-6.3,4.2
        c-0.9,0.6-1.8,0.9-2.6,0.9c-1,0-2-0.5-3-1.4C76.2,90,75,88.4,74,86.8c-1-1.7-2-3.6-3.1-5.9c-7.8,9.2-17.6,13.8-29.4,13.8
        c-8.4,0-15.1-2.4-20-7.2c-4.9-4.8-7.4-11.2-7.4-19.2c0-8.5,3-15.4,9.1-20.6c6.1-5.2,14.2-7.8,24.5-7.8c3.4,0,6.9,0.3,10.6,0.8
        c3.7,0.5,7.5,1.3,11.5,2.2v-7.3c0-7.6-1.6-12.9-4.7-16c-3.2-3.1-8.6-4.6-16.3-4.6c-3.5,0-7.1,0.4-10.8,1.3c-3.7,0.9-7.3,2-10.8,3.4
        c-1.6,0.7-2.8,1.1-3.5,1.3c-0.7,0.2-1.2,0.3-1.6,0.3c-1.4,0-2.1-1-2.1-3.1v-4.9c0-1.6,0.2-2.8,0.7-3.5c0.5-0.7,1.4-1.4,2.8-2.1
        c3.5-1.8,7.7-3.3,12.6-4.5c4.9-1.3,10.1-1.9,15.6-1.9c11.9,0,20.6,2.7,26.2,8.1c5.5,5.4,8.3,13.6,8.3,24.6V66.4z M45.8,81.6
        c3.3,0,6.7-0.6,10.3-1.8c3.6-1.2,6.8-3.4,9.5-6.4c1.6-1.9,2.8-4,3.4-6.4c0.6-2.4,1-5.3,1-8.7v-4.2c-2.9-0.7-6-1.3-9.2-1.7
        c-3.2-0.4-6.3-0.6-9.4-0.6c-6.7,0-11.6,1.3-14.9,4c-3.3,2.7-4.9,6.5-4.9,11.5c0,4.7,1.2,8.2,3.7,10.6
        C37.7,80.4,41.2,81.6,45.8,81.6z M126.1,92.4c-1.8,0-3-0.3-3.8-1c-0.8-0.6-1.5-2-2.1-3.9L96.7,10.2c-0.6-2-0.9-3.3-0.9-4
        c0-1.6,0.8-2.5,2.4-2.5h9.8c1.9,0,3.2,0.3,3.9,1c0.8,0.6,1.4,2,2,3.9l16.8,66.2l15.6-66.2c0.5-2,1.1-3.3,1.9-3.9c0.8-0.6,2.2-1,4-1
        h8c1.9,0,3.2,0.3,4,1c0.8,0.6,1.5,2,1.9,3.9l15.8,67l17.3-67c0.6-2,1.3-3.3,2-3.9c0.8-0.6,2.1-1,3.9-1h9.3c1.6,0,2.5,0.8,2.5,2.5
        c0,0.5-0.1,1-0.2,1.6c-0.1,0.6-0.3,1.4-0.7,2.5l-24.1,77.3c-0.6,2-1.3,3.3-2.1,3.9c-0.8,0.6-2.1,1-3.8,1h-8.6c-1.9,0-3.2-0.3-4-1
        c-0.8-0.7-1.5-2-1.9-4L156,23l-15.4,64.4c-0.5,2-1.1,3.3-1.9,4c-0.8,0.7-2.2,1-4,1H126.1z M254.6,95.1c-5.2,0-10.4-0.6-15.4-1.8
        c-5-1.2-8.9-2.5-11.5-4c-1.6-0.9-2.7-1.9-3.1-2.8c-0.4-0.9-0.6-1.9-0.6-2.8v-5.1c0-2.1,0.8-3.1,2.3-3.1c0.6,0,1.2,0.1,1.8,0.3
        c0.6,0.2,1.5,0.6,2.5,1c3.4,1.5,7.1,2.7,11,3.5c4,0.8,7.9,1.2,11.9,1.2c6.3,0,11.2-1.1,14.6-3.3c3.4-2.2,5.2-5.4,5.2-9.5
        c0-2.8-0.9-5.1-2.7-7c-1.8-1.9-5.2-3.6-10.1-5.2L246,52c-7.3-2.3-12.7-5.7-16-10.2c-3.3-4.4-5-9.3-5-14.5c0-4.2,0.9-7.9,2.7-11.1
        c1.8-3.2,4.2-6,7.2-8.2c3-2.3,6.4-4,10.4-5.2c4-1.2,8.2-1.7,12.6-1.7c2.2,0,4.5,0.1,6.7,0.4c2.3,0.3,4.4,0.7,6.5,1.1
        c2,0.5,3.9,1,5.7,1.6c1.8,0.6,3.2,1.2,4.2,1.8c1.4,0.8,2.4,1.6,3,2.5c0.6,0.8,0.9,1.9,0.9,3.3v4.7c0,2.1-0.8,3.2-2.3,3.2
        c-0.8,0-2.1-0.4-3.8-1.2c-5.7-2.6-12.1-3.9-19.2-3.9c-5.7,0-10.2,0.9-13.3,2.8c-3.1,1.9-4.7,4.8-4.7,8.9c0,2.8,1,5.2,3,7.1
        c2,1.9,5.7,3.8,11,5.5l14.2,4.5c7.2,2.3,12.4,5.5,15.5,9.6c3.1,4.1,4.6,8.8,4.6,14c0,4.3-0.9,8.2-2.6,11.6
        c-1.8,3.4-4.2,6.4-7.3,8.8c-3.1,2.5-6.8,4.3-11.1,5.6C264.4,94.4,259.7,95.1,254.6,95.1z"
        />
        <g fillRule="evenodd" clipRule="evenodd">
          <path
            d="M273.5,143.7c-32.9,24.3-80.7,37.2-121.8,37.2c-57.6,0-109.5-21.3-148.7-56.7c-3.1-2.8-0.3-6.6,3.4-4.4
          c42.4,24.6,94.7,39.5,148.8,39.5c36.5,0,76.6-7.6,113.5-23.2C274.2,133.6,278.9,139.7,273.5,143.7z"
          />
          <path
            d="M287.2,128.1c-4.2-5.4-27.8-2.6-38.5-1.3c-3.2,0.4-3.7-2.4-0.8-4.5c18.8-13.2,49.7-9.4,53.3-5
          c3.6,4.5-1,35.4-18.6,50.2c-2.7,2.3-5.3,1.1-4.1-1.9C282.5,155.7,291.4,133.4,287.2,128.1z"
          />
        </g>
      </g>
    </svg>
  );

  export const slackLogo: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 54 54">
      <g fill="none" fillRule="evenodd">
        <path
          d="M19.712.133a5.381 5.381 0 0 0-5.376 5.387 5.381 5.381 0 0 0 5.376 5.386h5.376V5.52A5.381 5.381 0 0 0 19.712.133m0 14.365H5.376A5.381 5.381 0 0 0 0 19.884a5.381 5.381 0 0 0 5.376 5.387h14.336a5.381 5.381 0 0 0 5.376-5.387 5.381 5.381 0 0 0-5.376-5.386"
          fill="#36C5F0"
        />
        <path
          d="M53.76 19.884a5.381 5.381 0 0 0-5.376-5.386 5.381 5.381 0 0 0-5.376 5.386v5.387h5.376a5.381 5.381 0 0 0 5.376-5.387m-14.336 0V5.52A5.381 5.381 0 0 0 34.048.133a5.381 5.381 0 0 0-5.376 5.387v14.364a5.381 5.381 0 0 0 5.376 5.387 5.381 5.381 0 0 0 5.376-5.387"
          fill="#2EB67D"
        />
        <path
          d="M34.048 54a5.381 5.381 0 0 0 5.376-5.387 5.381 5.381 0 0 0-5.376-5.386h-5.376v5.386A5.381 5.381 0 0 0 34.048 54m0-14.365h14.336a5.381 5.381 0 0 0 5.376-5.386 5.381 5.381 0 0 0-5.376-5.387H34.048a5.381 5.381 0 0 0-5.376 5.387 5.381 5.381 0 0 0 5.376 5.386"
          fill="#ECB22E"
        />
        <path
          d="M0 34.249a5.381 5.381 0 0 0 5.376 5.386 5.381 5.381 0 0 0 5.376-5.386v-5.387H5.376A5.381 5.381 0 0 0 0 34.25m14.336-.001v14.364A5.381 5.381 0 0 0 19.712 54a5.381 5.381 0 0 0 5.376-5.387V34.25a5.381 5.381 0 0 0-5.376-5.387 5.381 5.381 0 0 0-5.376 5.387"
          fill="#E01E5A"
        />
      </g>
    </svg>
  );

  export const slackLogoMono: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 128 128" fill="currentColor">
      <path d="M26.9,80.9c0,7.4-6,13.4-13.4,13.4S0,88.3,0,80.9c0-7.4,6-13.4,13.4-13.4h13.4V80.9z" />
      <path d="M33.7,80.9c0-7.4,6-13.4,13.4-13.4s13.4,6,13.4,13.4v33.7c0,7.4-6,13.4-13.4,13.4s-13.4-6-13.4-13.4 C33.7,114.6,33.7,80.9,33.7,80.9z" />
      <path d="M47.1,26.9c-7.4,0-13.4-6-13.4-13.4S39.7,0,47.1,0s13.4,6,13.4,13.4v13.4H47.1z" />
      <path d="M47.1,33.7c7.4,0,13.4,6,13.4,13.4s-6,13.4-13.4,13.4H13.4C6,60.6,0,54.5,0,47.1s6-13.4,13.4-13.4 C13.4,33.7,47.1,33.7,47.1,33.7z" />
      <path d="M101.1,47.1c0-7.4,6-13.4,13.4-13.4c7.4,0,13.4,6,13.4,13.4s-6,13.4-13.4,13.4h-13.4V47.1z" />
      <path d="M94.3,47.1c0,7.4-6,13.4-13.4,13.4c-7.4,0-13.4-6-13.4-13.4V13.4C67.4,6,73.5,0,80.9,0c7.4,0,13.4,6,13.4,13.4V47.1z" />
      <path d="M80.9,101.1c7.4,0,13.4,6,13.4,13.4c0,7.4-6,13.4-13.4,13.4c-7.4,0-13.4-6-13.4-13.4v-13.4H80.9z" />
      <path d="M80.9,94.3c-7.4,0-13.4-6-13.4-13.4c0-7.4,6-13.4,13.4-13.4h33.7c7.4,0,13.4,6,13.4,13.4c0,7.4-6,13.4-13.4,13.4H80.9z" />
    </svg>
  );

  export const twitterLogo: Icon = (cls, title) => (
    <svg className={cls} viewBox="0 0 248 204" fill="currentColor">
      <path
        d="M221.95,51.29c0.15,2.17,0.15,4.34,0.15,6.53c0,66.73-50.8,143.69-143.69,143.69v-0.04
          C50.97,201.51,24.1,193.65,1,178.83c3.99,0.48,8,0.72,12.02,0.73c22.74,0.02,44.83-7.61,62.72-21.66
          c-21.61-0.41-40.56-14.5-47.18-35.07c7.57,1.46,15.37,1.16,22.8-0.87C27.8,117.2,10.85,96.5,10.85,72.46c0-0.22,0-0.43,0-0.64
          c7.02,3.91,14.88,6.08,22.92,6.32C11.58,63.31,4.74,33.79,18.14,10.71c25.64,31.55,63.47,50.73,104.08,52.76
          c-4.07-17.54,1.49-35.92,14.61-48.25c20.34-19.12,52.33-18.14,71.45,2.19c11.31-2.23,22.15-6.38,32.07-12.26
          c-3.77,11.69-11.66,21.62-22.2,27.93c10.01-1.18,19.79-3.86,29-7.95C240.37,35.29,231.83,44.14,221.95,51.29z"
      />
    </svg>
  );

  export const loading = (cls: string, color: string, baseColor: string, borderWidth: number) => (
    <>
      <style>{`
        .loading {
          -webkit-animation: spinner 0.5s linear infinite;
          animation: spinner 0.5s linear infinite;
          border-color: ${baseColor};
        }

        @-webkit-keyframes spinner {
          0% {
            -webkit-transform: rotate(0deg);
          }
          100% {
            -webkit-transform: rotate(360deg);
          }
        }

        @keyframes spinner {
          0% {
            transform: rotate(0deg);
          }
          100% {
            transform: rotate(360deg);
          }
        }
      `}</style>
      <div
        className={`inline-block ${cls} loading rounded-full border-transparent ease-linear`}
        style={{ borderWidth, borderTopColor: color }}
      />
    </>
  );

  const renderTitle = (title?: string) => title && <title>{title}</title>;
}
