import { HomeIcon } from "@heroicons/react/20/solid";
import React, { FunctionComponent } from "react";
import { Link, Outlet, useParams } from "react-router-dom";
import { NavHashLink } from "~c/HashLink";
import Nav from "~c/Nav";
import { snippetData, SnippetSection } from "~c/snippets/snippetData";

const getAnchorFromHeader = (header: string) => header.toLowerCase().split(" ").join("-");

export const SnippetPage: FunctionComponent = () => {
  const { appID, slug } = useParams();
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
                          <NavHashLink
                            to={{
                              pathname: `/${appID}/snippets/${section.slug}`,
                            }}
                          >
                            {section.heading}
                          </NavHashLink>
                        </div>
                      </div>
                    </button>

                    <div className="space-y-1 text-sm">
                      {section.subSections.map((subSection) => (
                        <NavHashLink
                          key={subSection.heading}
                          to={{
                            pathname: `/${appID}/snippets/${section.slug}`,
                            hash: `#${getAnchorFromHeader(subSection.heading)}`,
                          }}
                          className="block py-1 pl-4 focus:outline-none"
                        >
                          {subSection.heading}
                        </NavHashLink>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>
            <div className="flex h-full-minus-nav min-w-0 flex-1 flex-col overflow-auto">
              <div className="min-h-0 flex-grow p-8 leading-6">
                <Outlet />
                {slug === undefined && (
                  <div className="grid w-full max-w-5xl grid-cols-1 gap-8 md:grid-cols-2">
                    {snippetData.map((section) => {
                      const Icon = section.icon;
                      return (
                        <Link to={section.slug} className="group relative block">
                          <div className="absolute inset-0 -z-10 bg-black dark:bg-white"></div>
                          <div className="relative min-h-full border border-black bg-white p-8 transition-transform duration-100 ease-in-out group-hover:-translate-x-2 group-hover:-translate-y-2 group-active:-translate-x-2 group-active:-translate-y-2 dark:border-white dark:bg-black mobile:p-4">
                            <div className="flex items-center justify-between">
                              <h3 className="text-lg font-medium">{section.heading}</h3>
                              <Icon className="-mt-2 h-8 w-8" />
                            </div>
                            <p className="mt-2">
                              Learn about what problems Encore solves and the philosophy behind it.
                            </p>
                          </div>
                        </Link>
                      );
                    })}
                  </div>
                )}
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
  if (!section) return null;

  return (
    <div className="max-w-4xl">
      <h1 className="mb-5 text-3xl">{section.heading}</h1>
      {section.description && <div className="mb-8 flex flex-col gap-4">{section.description}</div>}

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
};
