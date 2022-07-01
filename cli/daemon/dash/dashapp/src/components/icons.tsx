import React from "react"

export type Icon = (cls?: string, title?: string) => JSX.Element

export const commit: Icon = (cls, title): JSX.Element =>
  <svg className={cls} fill="currentColor" viewBox="0 0 896 1024">
    {renderTitle(title)}
    <path d="M694.875 448C666.375 337.781 567.125 256 448 256c-119.094 0-218.375 81.781-246.906 192H0v128h201.094C229.625 686.25 328.906 768 448 768c119.125 0 218.375-81.75 246.875-192H896V448H694.875zM448 640c-70.656 0-128-57.375-128-128 0-70.656 57.344-128 128-128 70.625 0 128 57.344 128 128C576 582.625 518.625 640 448 640z" />
  </svg>

export const lightningBolt: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path d="M13 10V3L4 14h7v7l9-11h-7z" />
  </svg>

export const lightBulb: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
  </svg>

export const code: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
  </svg>

export const cloud: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path d="M 8 18.999 C 4.151 19.002 1.743 14.837 3.665 11.502 C 4.396 10.234 5.645 9.35 7.084 9.083 C 7.795 5.299 12.336 3.703 15.258 6.211 C 16.121 6.952 16.706 7.965 16.916 9.083 C 20.699 9.802 22.285 14.346 19.771 17.263 C 18.825 18.36 17.449 18.994 16 18.999 Z" />
  </svg>

export const cloudUpload: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M8 17a5 5 0 01-.916-9.916 5.002 5.002 0 019.832 0A5.002 5.002 0 0116 17m-7-5l3-3m0 0l3 3m-3-3v12" />
  </svg>

export const document: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
  </svg>

export const documentReport: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
  </svg>

export const documentDuplicate: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2" />
  </svg>

export const solidDocument: Icon = (cls, title) =>
  <svg className={cls} fill="currentColor" viewBox="0 0 24 24 ">
    {renderTitle(title)}
    <path fillRule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" clipRule="evenodd" />
  </svg>

export const pencil: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
  </svg>

export const merge: Icon = (cls, title) =>
  <svg className={cls} fill="currentColor" viewBox="0 0 768 1024">
    {renderTitle(title)}
    <path d="M640 448c-47.625 0-88.625 26.312-110.625 64.906C523.625 512.5 518 512 512 512c-131.062 0-255.438-99.844-300.812-223.438C238.469 265.09400000000005 256 230.71900000000005 256 192c0-70.656-57.344-128-128-128S0 121.34400000000005 0 192c0 47.219 25.844 88.062 64 110.281V721.75C25.844 743.938 0 784.75 0 832c0 70.625 57.344 128 128 128s128-57.375 128-128c0-47.25-25.844-88.062-64-110.25V491.469C276.156 580.5 392.375 640 512 640c6.375 0 11.625-0.438 17.375-0.625C551.5 677.812 592.5 704 640 704c70.625 0 128-57.375 128-128C768 505.344 710.625 448 640 448zM128 896c-35.312 0-64-28.625-64-64 0-35.312 28.688-64 64-64 35.406 0 64 28.688 64 64C192 867.375 163.406 896 128 896zM128 256c-35.312 0-64-28.594-64-64s28.688-64 64-64c35.406 0 64 28.594 64 64S163.406 256 128 256zM640 640c-35.312 0-64-28.625-64-64 0-35.406 28.688-64 64-64 35.375 0 64 28.594 64 64C704 611.375 675.375 640 640 640z"/>
  </svg>

export const calendar: Icon = (cls, title) =>
  <svg className={cls} fill="currentColor" viewBox="0 0 20 20">
    {renderTitle(title)}
    <path fillRule="evenodd" d="M6 2a1 1 0 00-1 1v1H4a2 2 0 00-2 2v10a2 2 0 002 2h12a2 2 0 002-2V6a2 2 0 00-2-2h-1V3a1 1 0 10-2 0v1H7V3a1 1 0 00-1-1zm0 5a1 1 0 000 2h8a1 1 0 100-2H6z" clipRule="evenodd"/>
  </svg>

export const search: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
  </svg>

export const chevronDown: Icon = (cls, title) =>
  <svg className={cls} fill="currentColor" viewBox="0 0 20 20">
    {renderTitle(title)}
    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd"/>
  </svg>

export const chevronRight: Icon = (cls, title) =>
  <svg className={cls} fill="currentColor" viewBox="0 0 20 20">
    {renderTitle(title)}
    <path transform="rotate(-90 10 10)" fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd"/>
  </svg>

export const x: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path d="M6 18L18 6M6 6l12 12" />
  </svg>

export const check: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M5 13l4 4L19 7" />
  </svg>

export const checkCircle: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>

export const xCircle: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>

export const exclamation: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
  </svg>

export const exclamationCircle: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>

export const playCircle: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
  </svg>

export const questionMarkCircle: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>

export const dotsCircle: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M8 12h.01M12 12h.01M16 12h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>

export const plus: Icon = (cls, title) =>
  <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
  </svg>

export const minus: Icon = (cls, title) =>
  <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 12H4" />
  </svg>

export const plusCircle: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M12 9v3m0 0v3m0-3h3m-3 0H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>

export const minusCircle: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M15 12H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>

export const stackTrace: Icon = (cls, title) =>
  <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h8m-8 6h12" />
  </svg>

export const refresh: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
  </svg>

export const chip: Icon = (cls, title) =>
  <svg className={cls} viewBox="0 0 20 20" fill="currentColor">
    {renderTitle(title)}
    <path d="M13 7H7v6h6V7z" />
    <path fillRule="evenodd" d="M7 2a1 1 0 012 0v1h2V2a1 1 0 112 0v1h2a2 2 0 012 2v2h1a1 1 0 110 2h-1v2h1a1 1 0 110 2h-1v2a2 2 0 01-2 2h-2v1a1 1 0 11-2 0v-1H9v1a1 1 0 11-2 0v-1H5a2 2 0 01-2-2v-2H2a1 1 0 110-2h1V9H2a1 1 0 010-2h1V5a2 2 0 012-2h2V2zM5 5h10v10H5V5z" clipRule="evenodd" />
  </svg>

export const chevronLeft: Icon = (cls, title) =>
  <svg className={cls} fill="currentColor" viewBox="0 0 20 20">
    {renderTitle(title)}
    <path transform="rotate(90 10 10)" fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd"/>
  </svg>

export const clock: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>

export const puzzle: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M11 4a2 2 0 114 0v1a1 1 0 001 1h3a1 1 0 011 1v3a1 1 0 01-1 1h-1a2 2 0 100 4h1a1 1 0 011 1v3a1 1 0 01-1 1h-3a1 1 0 01-1-1v-1a2 2 0 10-4 0v1a1 1 0 01-1 1H7a1 1 0 01-1-1v-3a1 1 0 00-1-1H4a2 2 0 110-4h1a1 1 0 001-1V7a1 1 0 011-1h3a1 1 0 001-1V4z" />
  </svg>

export const filter: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
  </svg>

export const slash: Icon = (cls, title) =>
  <svg className={cls} viewBox="0 0 24 24" stroke="currentColor" strokeWidth="1" strokeLinecap="round" strokeLinejoin="round" fill="none" shapeRendering="geometricPrecision">
    {renderTitle(title)}
    <path d="M16.88 3.549L7.12 20.451" />
  </svg>

export const selector: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
  </svg>

export const user: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
  </svg>

export const userAdd: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z" />
  </svg>

export const userRemove: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M13 7a4 4 0 11-8 0 4 4 0 018 0zM9 14a6 6 0 00-6 6v1h12v-1a6 6 0 00-6-6zM21 12h-6" />
  </svg>

export const userCircle: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M5.121 17.804A13.937 13.937 0 0112 16c2.5 0 4.847.655 6.879 1.804M15 10a3 3 0 11-6 0 3 3 0 016 0zm6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>

export const logout: Icon = (cls, title) =>
  <svg className={cls} viewBox="0 0 20 20" fill="currentColor">
    {renderTitle(title)}
    <path fillRule="evenodd" d="M3 3a1 1 0 00-1 1v12a1 1 0 102 0V4a1 1 0 00-1-1zm10.293 9.293a1 1 0 001.414 1.414l3-3a1 1 0 000-1.414l-3-3a1 1 0 10-1.414 1.414L14.586 9H7a1 1 0 100 2h7.586l-1.293 1.293z" clipRule="evenodd" />
  </svg>

export const key: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
  </svg>

export const wrench: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <path d="M 10.78 9.805 L 1.5 19.009 L 4.999 22.497 L 14.26 13.209 C 16.438 14.027 19.205 13.797 20.962 12.048 C 22.858 10.158 22.853 6.986 21.75 4.699 L 17.657 8.779 L 15.205 6.336 L 19.298 2.256 C 17.014 1.146 13.811 1.141 11.916 3.03 C 10.133 4.807 9.911 7.605 10.78 9.805 Z" />
  </svg>

export const database: Icon = (cls, title) =>
  <svg className={cls} viewBox="0 0 20 20" fill="currentColor">
    {renderTitle(title)}
    <path d="M3 12v3c0 1.657 3.134 3 7 3s7-1.343 7-3v-3c0 1.657-3.134 3-7 3s-7-1.343-7-3z" />
    <path d="M3 7v3c0 1.657 3.134 3 7 3s7-1.343 7-3V7c0 1.657-3.134 3-7 3S3 8.657 3 7z" />
    <path d="M17 5c0 1.657-3.134 3-7 3S3 6.657 3 5s3.134-3 7-3 7 1.343 7 3z" />
  </svg>

export const menuAlt2: Icon = (cls, title) =>
  <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    {renderTitle(title)}
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16M4 18h7" />
  </svg>

export const errCircle: Icon = (cls, title) =>
  <svg className={cls} viewBox="0 0 20 20" fill="currentColor">
    {renderTitle(title)}
    <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
  </svg>

export const photograph: Icon = (cls, title) =>
  <svg className={cls} viewBox="0 0 20 20" fill="currentColor">
    {renderTitle(title)}
    <path fillRule="evenodd" d="M4 3a2 2 0 00-2 2v10a2 2 0 002 2h12a2 2 0 002-2V5a2 2 0 00-2-2H4zm12 12H4l4-8 3 6 2-4 3 6z" clipRule="evenodd" />
  </svg>

export const pulse: Icon = (cls, title) =>
  <svg className={cls} fill="none" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" stroke="currentColor">
    {renderTitle(title)}
    <polyline points="2 14.308 5.076 14.308 8.154 2 11.231 20.462 14.308 9.692 15.846 14.308 18.924 14.308" />
    <circle cx="20.462" cy="14.308" r="1.538" />
  </svg>

export const githubLogo: Icon = (cls, title) =>
  <svg className={cls} viewBox="0 0 16 16" fill="currentColor">
    {renderTitle(title)}
    <path fillRule="evenodd" d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
  </svg>

export const azureLogo: Icon = (cls, title) =>
  <svg className={cls} viewBox="0 0 19.7 15.2" fill="currentColor">
    {renderTitle(title)}
    <path d="M 9.105 14.431 C 11.634 13.984 13.723 13.614 13.747 13.609 L 13.79 13.6 L 11.403 10.76 C 10.09 9.199 9.016 7.915 9.016 7.907 C 9.016 7.893 11.481 1.105 11.495 1.081 C 11.499 1.073 13.177 3.969 15.561 8.102 C 17.793 11.971 19.634 15.161 19.651 15.191 L 19.682 15.245 L 12.095 15.244 L 4.508 15.243 L 9.105 14.431 Z M 0 13.565 C 0 13.561 1.125 11.608 2.5 9.225 L 5 4.893 L 7.913 2.448 C 9.515 1.104 10.83 0.002 10.836 0 C 10.841 -0.002 10.82 0.051 10.789 0.118 C 10.758 0.185 9.334 3.238 7.625 6.903 L 4.518 13.566 L 2.259 13.569 C 1.017 13.571 0 13.569 0 13.565 Z" />
  </svg>

export const encoreLogo: Icon = (cls, title) =>
  <svg className={cls} viewBox="0 0 243 203" fill="currentColor">
    {renderTitle(title)}
    <path fillRule="evenodd" d="M0,175 C0,82.9190589 73,0 175,0 C175,0 175,67 175,67 C111.870929,67 67,117.841363 67,175 C66.8660649,175 0,175 0,175 Z" />
    <path fillRule="evenodd" d="M175.279086,107.763158 C214.190992,107.763158 242.731481,138.782778 242.731481,175.001086 C242.800404,196.117137 201.758492,202.631579 175.11111,202.631579 C148.463728,202.631579 107.615418,196.151462 107.546296,175.001086 C107.546296,138.251969 136.367179,107.763158 175.279086,107.763158 Z" />
  </svg>

export const gcpLogo: Icon = (cls, title) =>
  <svg className={cls} viewBox="0 0 18 14.5">
    {renderTitle(title)}
    <path fill="#EA4335" d="M 11.414 3.993 L 11.963 3.993 L 13.527 2.428 L 13.604 1.764 C 10.692 -0.807 6.248 -0.529 3.678 2.383 C 2.963 3.191 2.446 4.154 2.163 5.195 C 2.337 5.124 2.53 5.112 2.712 5.163 L 5.841 4.646 C 5.841 4.646 6 4.383 6.083 4.399 C 7.474 2.87 9.817 2.692 11.425 3.993 L 11.414 3.993 Z" />
    <path fill="#4285F4" d="M 15.757 5.195 C 15.397 3.871 14.659 2.68 13.632 1.77 L 11.437 3.966 C 12.364 4.723 12.892 5.865 12.869 7.062 L 12.869 7.452 C 13.949 7.452 14.824 8.328 14.824 9.407 C 14.824 10.486 13.949 11.362 12.869 11.362 L 8.96 11.362 L 8.57 11.757 L 8.57 14.102 L 8.96 14.492 L 12.869 14.492 C 15.677 14.514 17.97 12.255 17.992 9.448 C 18.005 7.743 17.165 6.148 15.757 5.195 Z" />
    <path fill="#34A853" d="M 5.046 14.467 L 8.955 14.467 L 8.955 11.338 L 5.046 11.338 C 4.767 11.338 4.492 11.277 4.238 11.162 L 3.69 11.333 L 2.114 12.897 L 1.977 13.446 C 2.86 14.114 3.938 14.472 5.046 14.467 Z" />
    <path fill="#FBBC05" d="M 5.046 4.317 C 2.237 4.333 -0.024 6.623 -0.008 9.431 C 0.002 10.999 0.733 12.475 1.977 13.43 L 4.244 11.163 C 3.261 10.719 2.823 9.56 3.267 8.577 C 3.711 7.594 4.869 7.156 5.853 7.6 C 6.286 7.796 6.633 8.144 6.829 8.577 L 9.096 6.31 C 8.133 5.048 6.633 4.31 5.046 4.317 Z" />
  </svg>

export const inbox: Icon = (cls, title) =>
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
        {renderTitle(title)}
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2"
              d="M8 4H6a2 2 0 00-2 2v12a2 2 0 002 2h12a2 2 0 002-2V6a2 2 0 00-2-2h-2m-4-1v8m0 0l3-3m-3 3L9 8m-5 5h2.586a1 1 0 01.707.293l2.414 2.414a1 1 0 00.707.293h3.172a1 1 0 00.707-.293l2.414-2.414a1 1 0 01.707-.293H20"></path>
    </svg>

export const globe: Icon = (cls, title) =>
    <svg className={cls} fill="none" stroke="currentColor" viewBox="0 0 24 24">
        {renderTitle(title)}
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2"
              d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
    </svg>

export const shield: Icon = (cls, title) =>
    <svg className={cls}  fill="none" stroke="currentColor" viewBox="0 0 24 24">
        {renderTitle(title)}
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2"
              d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"></path>
    </svg>

export const loading = (cls: string, color: string, baseColor: string, borderWidth: number) => (
  <>
    <style>{`
      .loading {
        -webkit-animation: spinner 0.5s linear infinite;
        animation: spinner 0.5s linear infinite;
        border-color: ${baseColor};
      }

      @-webkit-keyframes spinner {
        0% { -webkit-transform: rotate(0deg); }
        100% { -webkit-transform: rotate(360deg); }
      }

      @keyframes spinner {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
      }
    `}</style>
    <div className={`inline-block ${cls} border-transparent loading ease-linear rounded-full`}
        style={{borderWidth, borderTopColor: color}} />
  </>
)

const renderTitle = (title?: string) => (
  title && <title>{title}</title>
)
