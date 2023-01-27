import React, { FunctionComponent } from "react";
import Nav from "~c/Nav";
import { NavLink, Outlet, useParams } from "react-router-dom";
import { snippetData, SnippetSection } from "~c/snippets/snippetData";

const getAnchorFromHeader = (header: string) => header.toLowerCase().split(" ").join("-");

export const SnippetPage: FunctionComponent = () => {
  const { appID } = useParams();
  return (
    <>
      <Nav />

      <section className="flex flex-grow flex-col items-center">
        <div className="flex min-h-0 w-full flex-1 flex-col overflow-auto">
          <div className="flex min-w-0 flex-1 items-stretch overflow-auto">
            <div className="h-full-minus-nav w-64 flex-shrink-0 overflow-auto bg-black text-white lg:flex lg:flex-col">
              <div className="flex min-h-0 flex-1 flex-col gap-4 p-4">
                {snippetData.map((section) => (
                  <div key={section.heading}>
                    <button
                      type="button"
                      className="flex w-full items-center px-2 py-1 text-left focus:outline-none"
                    >
                      <div className="flex flex-grow items-center">
                        <div className="flex-grow font-semibold leading-5 text-white">
                          {section.heading}
                        </div>
                      </div>
                    </button>

                    <div className="space-y-1 text-sm">
                      {section.subSections.map((subSection) => (
                        <NavLink
                          key={subSection.heading}
                          to={{
                            pathname: `/${appID}/snippets/${section.slug}`,
                            hash: `#${getAnchorFromHeader(subSection.heading)}`,
                          }}
                          className="block py-1 pl-4 focus:outline-none"
                        >
                          {subSection.heading}
                        </NavLink>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>
            <div className="flex h-full-minus-nav min-w-0 flex-1 flex-col overflow-auto">
              <div className="min-h-0 flex-grow p-4 leading-6">
                <Outlet />
              </div>
            </div>
          </div>
        </div>
      </section>
    </>
  );
};

export const SnippetContent: FunctionComponent = () => {
  const { slug } = useParams();
  const section: SnippetSection | undefined = snippetData.find((s) => s.slug === slug);
  if (section)
    return (
      <div className="max-w-4xl">
        <h1 className="mb-5 text-3xl">{section.heading}</h1>

        <div className="space-y-10 pb-10">
          {section.subSections.map((section) => (
            <div key={section.heading}>
              <h2 className="mb-4 text-2xl" id={getAnchorFromHeader(section.heading)}>
                {section.heading}
              </h2>
              {section.content}
            </div>
          ))}
        </div>
      </div>
    );
  return null;
};
