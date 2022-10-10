import React from "react";
import { render, screen, waitFor } from "@testing-library/react";
import SpanDetail from "~c/trace/SpanDetail";
import userEvent from "@testing-library/user-event";

describe("SpanDetails", () => {
  it("should show correctly styled SQL code when hovering over trace", async () => {
    const trace = {
      id: "3c326bf5-9742-13b6-c6c7-a6f867390aed",
      date: "2022-09-21T13:40:52.031442+02:00",
      start_time: 0,
      end_time: 818993,
      root: {
        id: "4922198311992459089",
        type: "RPC",
        parent_id: null,
        goid: 1,
        start_time: 0,
        end_time: 818993,
        svc_name: "user",
        rpc_name: "Read",
        topic_name: "",
        subscriber_name: "",
        msg_id: "",
        attempt: 0,
        published: null,
        call_loc: null,
        def_loc: 32,
        inputs: ["IjprZXki"],
        outputs: ["ewogICJLZXkiOiAiOmtleSIsCiAgIlZhbHVlIjogIjp2YWx1ZSIKfQ=="],
        err: null,
        err_stack: null,
        events: [
          {
            type: "DBQuery",
            goid: 1,
            txid: null,
            call_loc: 0,
            start_time: 4041,
            end_time: 816712,
            query: "U0VMRUNUIGtleSwgdmFsdWUgRlJPTSAia2V5X3ZhbHVlcyIgV0hFUkUga2V5PSQx",
            html_query:
              "PHByZSB0YWJpbmRleD0iMCIgc3R5bGU9ImJhY2tncm91bmQtY29sb3I6I2ZmZjsiPjxjb2RlPjxzcGFuIHN0eWxlPSJkaXNwbGF5OmZsZXg7Ij48c3Bhbj48c3BhbiBzdHlsZT0iY29sb3I6IzAwZiI+U0VMRUNUPC9zcGFuPiA8c3BhbiBzdHlsZT0iY29sb3I6IzAwZiI+a2V5PC9zcGFuPiwgPHNwYW4gc3R5bGU9ImNvbG9yOiMwMGYiPnZhbHVlPC9zcGFuPiA8c3BhbiBzdHlsZT0iY29sb3I6IzAwZiI+RlJPTTwvc3Bhbj4gPHNwYW4gc3R5bGU9ImNvbG9yOiNhMzE1MTUiPjwvc3Bhbj48c3BhbiBzdHlsZT0iY29sb3I6I2EzMTUxNSI+JiMzNDs8L3NwYW4+PHNwYW4gc3R5bGU9ImNvbG9yOiNhMzE1MTUiPmtleV92YWx1ZXM8L3NwYW4+PHNwYW4gc3R5bGU9ImNvbG9yOiNhMzE1MTUiPiYjMzQ7PC9zcGFuPiA8c3BhbiBzdHlsZT0iY29sb3I6IzAwZiI+V0hFUkU8L3NwYW4+IDxzcGFuIHN0eWxlPSJjb2xvcjojMDBmIj5rZXk8L3NwYW4+PSQxPC9zcGFuPjwvc3Bhbj48L2NvZGU+PC9wcmU+",
            err: null,
            stack: {
              frames: [],
            },
          },
        ],
        children: [],
      },
      auth: null,
      uid: null,
      user_data: null,
      locations: {
        "32": {
          id: 32,
          filepath: "user/read.go",
          start_pos: 513,
          end_pos: 578,
          src_line_start: 33,
          src_line_end: 33,
          src_col_start: 1,
          src_col_end: 66,
          rpc_def: {
            service_name: "user",
            rpc_name: "Read",
            context: "func Read(ctx context.Context, key string) (*KeyValuePair, error)",
          },
        },
      },
      meta: {
        svcs: [],
      },
    };
    const req = {
      id: "4922198311992459089",
      type: "RPC",
      parent_id: null,
      goid: 1,
      start_time: 0,
      end_time: 818993,
      svc_name: "user",
      rpc_name: "Read",
      topic_name: "",
      subscriber_name: "",
      msg_id: "",
      attempt: 0,
      published: null,
      call_loc: null,
      def_loc: 32,
      inputs: ["IjprZXki"],
      outputs: ["ewogICJLZXkiOiAiOmtleSIsCiAgIlZhbHVlIjogIjp2YWx1ZSIKfQ=="],
      err: null,
      err_stack: null,
      events: [
        {
          type: "DBQuery",
          goid: 1,
          txid: null,
          call_loc: 0,
          start_time: 4041,
          end_time: 816712,
          query: "U0VMRUNUIGtleSwgdmFsdWUgRlJPTSAia2V5X3ZhbHVlcyIgV0hFUkUga2V5PSQx",
          html_query:
            "PHByZSB0YWJpbmRleD0iMCIgc3R5bGU9ImJhY2tncm91bmQtY29sb3I6I2ZmZjsiPjxjb2RlPjxzcGFuIHN0eWxlPSJkaXNwbGF5OmZsZXg7Ij48c3Bhbj48c3BhbiBzdHlsZT0iY29sb3I6IzAwZiI+U0VMRUNUPC9zcGFuPiA8c3BhbiBzdHlsZT0iY29sb3I6IzAwZiI+a2V5PC9zcGFuPiwgPHNwYW4gc3R5bGU9ImNvbG9yOiMwMGYiPnZhbHVlPC9zcGFuPiA8c3BhbiBzdHlsZT0iY29sb3I6IzAwZiI+RlJPTTwvc3Bhbj4gPHNwYW4gc3R5bGU9ImNvbG9yOiNhMzE1MTUiPjwvc3Bhbj48c3BhbiBzdHlsZT0iY29sb3I6I2EzMTUxNSI+JiMzNDs8L3NwYW4+PHNwYW4gc3R5bGU9ImNvbG9yOiNhMzE1MTUiPmtleV92YWx1ZXM8L3NwYW4+PHNwYW4gc3R5bGU9ImNvbG9yOiNhMzE1MTUiPiYjMzQ7PC9zcGFuPiA8c3BhbiBzdHlsZT0iY29sb3I6IzAwZiI+V0hFUkU8L3NwYW4+IDxzcGFuIHN0eWxlPSJjb2xvcjojMDBmIj5rZXk8L3NwYW4+PSQxPC9zcGFuPjwvc3Bhbj48L2NvZGU+PC9wcmU+",
          err: null,
          stack: {
            frames: [],
          },
        },
      ],
      children: [],
    };

    render(<SpanDetail trace={trace as any} req={req as any} onStackTrace={() => {}} />);

    const traceBar = screen.getByTestId("ev-4922198311992459089-1-0");
    await userEvent.hover(traceBar);

    await waitFor(() => {
      const tooltipEl = screen.getByTestId("trace-tooltip");
      const divEl = tooltipEl.querySelector(".CodeMirror.cm-s-encore");

      expect(divEl!.textContent).toEqual('SELECT key, value FROM "key_values" WHERE key=$1');
    });
  });
});
