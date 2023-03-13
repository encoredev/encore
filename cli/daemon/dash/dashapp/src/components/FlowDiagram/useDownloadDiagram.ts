import React, { useCallback } from "react";
import { toCanvas } from "html-to-image";

const useDownloadDiagram = (ref: React.RefObject<HTMLDivElement>) => {
  return useCallback(() => {
    if (ref.current === null) return;

    return toCanvas(ref.current, {
      cacheBust: true,
      backgroundColor: "#EEEEE1",
    })
      .then(watermarkCanvas)
      .then((canvas) => {
        const dataUrl = canvas.toDataURL();
        const link = document.createElement("a");
        link.download = "encore-flow.png";
        link.href = dataUrl;
        link.click();
      })
      .catch((err) => {
        console.log(err);
      });
  }, [ref]);
};

export default useDownloadDiagram;

const watermarkCanvas = (canvas: HTMLCanvasElement): Promise<HTMLCanvasElement> => {
  return new Promise((resolve) => {
    const context = canvas.getContext("2d");
    const encoreLockupLogoSvg = `
        <svg version="1.1" id="Encore_Lockup" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px"
           y="0px" viewBox="0 0 620.4 195.6" width="300" xml:space="preserve">
        <style type="text/css">
          .st0{fill:#111111;}
        </style>
        <g>
          <rect height="1000" width="1000" fill="#EEEEE1" />
          <path class="st0" d="M536,81.5c-10.4-0.1-16.8,6.3-15.2,22.1c9.1-3.6,18.4-7.9,27.7-13.1C545.8,82.4,538.8,81.5,536,81.5z M540,148
            c-26.3-0.2-42.6-17-42.6-44c0-26.7,15.2-43.3,39.7-43.3c20.6,0,33.5,12.9,37.4,37.3c-14.2,9.4-30,17.7-46.6,24.1
            c0,0,4.3,3.1,13.2,3h32.4V148H540z M288.2,148V98.6c0-11.1-3-14.3-13.3-14.3h-11V148h-23.1V61.7h36.7c22.7,0,33.7,10.4,33.7,31.9
            V148H288.2z M358.5,148c-26.4,0-42.8-16.3-42.8-42.9c0-26.8,16.3-43.3,42.6-43.3h10v23.6h-8.4c-13,0.2-20.4,7.4-20.4,19.8
            c0,12,7.4,19,20.4,19.1h8.8V148H358.5z M452.6,148V93.6c0-21.5,11-31.9,33.7-31.9H497v22.6h-8c-10.3,0-13.3,3.2-13.3,14.3V148
            H452.6z M198.2,81.5c-10.4-0.1-16.8,6.3-15.2,22.1c9.1-3.6,18.4-7.9,27.7-13.1C208,82.4,201,81.5,198.2,81.5z M202.2,148
            c-26.3-0.2-42.6-17-42.6-44c0-26.7,15.2-43.3,39.7-43.3c20.6,0,33.5,12.9,37.4,37.3c-14.2,9.4-30,17.7-46.6,24.1
            c0,0,4.3,3.1,13.2,3h32.4V148H202.2z M136.7,125V148H45.8V78.6c14.4-3.1,28.7-7.1,42.6-12c16.6-5.8,32.7-12.7,48.3-20.8v25.6
            c-13.2,6.4-26.9,12-40.8,16.9c-15.7,5.5-31.8,9.9-48.1,13.3v0.2c30.1-2.8,59.7-7.7,88.9-14.5v23.5c-29.2,6.6-58.9,11.3-88.9,14v0.2
            H136.7z M370.5,104.8c0,27.7,14.6,44.9,38.8,44.9s38.8-17.2,38.8-44.9c0.1-14.7-3.8-26.1-11.5-33.8c-6.7-6.8-16.4-10.5-27.3-10.5
            S388.7,64.2,382,71C374.3,78.8,370.4,90.2,370.5,104.8z M392.7,105.2c0-13.1,6.7-22,16.6-22c9.9,0,16.6,8.8,16.6,22
            c0,13.1-6.6,21.6-16.6,21.6C399.3,126.8,392.7,118.3,392.7,105.2z"/>
        </g>
        </svg>
      `;
    const svgBlob = new Blob([encoreLockupLogoSvg], { type: "image/svg+xml;charset=utf-8" }),
      domURL = self.URL || self.webkitURL || self,
      url = domURL.createObjectURL(svgBlob),
      img = new Image();

    img.onload = () => {
      const x = 20;
      const y = canvas.height - img.height - 20;
      context!.drawImage(img, x, y);
      domURL.revokeObjectURL(url);
      resolve(canvas);
    };

    img.src = url;
  });
};
