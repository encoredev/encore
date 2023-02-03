import React, { FC, FunctionComponent } from "react";
import { Link, Outlet, useParams } from "react-router-dom";
import { NavHashLink } from "~c/HashLink";
import Nav from "~c/Nav";
import { snippetData, SnippetSection } from "~c/snippets/snippetData";
import { CircleStackIcon, QuestionMarkCircleIcon } from "@heroicons/react/24/solid";

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
                          className="block py-1 pl-4 focus:outline-none hover:bg-white hover:bg-opacity-10"
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
                  <div className="w-full max-w-5xl">
                    <div className="mb-5">
                      When you're familiar with how Encore works, you can simplify your development
                      workflow by copy-pasting these examples. If you're looking for details on how
                      Encore works, please refer to the relevant docs section.
                    </div>
                    <div className="grid grid-cols-1 gap-8 md:grid-cols-2">
                      {snippetData.map((section) => {
                        const Icon = section.icon;
                        return (
                          <Link
                            key={section.slug}
                            to={section.slug}
                            className="group relative block"
                          >
                            <OverviewCard
                              heading={section.heading}
                              description="Learn about what problems Encore solves and the philosophy behind it."
                              icon={section.icon}
                            />
                          </Link>
                        );
                      })}
                      <a
                        target="_blank"
                        href="https://encoredev.slack.com/app_redirect?channel=CQFNUESN9"
                        className="group relative block"
                      >
                        <OverviewCard
                          heading="Something missing?"
                          description="Are you missing a snippet that would be useful to you? Let us know about
                            it!"
                          icon={QuestionMarkCircleIcon}
                        />
                      </a>
                    </div>
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

const OverviewCard: FC<{ heading: string; description: string; icon: typeof CircleStackIcon }> = (
  props
) => {
  return (
    <>
      <div className="absolute inset-0 -z-10 bg-black dark:bg-white"></div>
      <div className="relative min-h-full border border-black bg-white p-8 transition-transform duration-100 ease-in-out group-hover:-translate-x-2 group-hover:-translate-y-2 group-active:-translate-x-2 group-active:-translate-y-2 dark:border-white dark:bg-black mobile:p-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-medium">{props.heading}</h3>
          <props.icon className="-mt-2 h-8 w-8" />
        </div>
        <p className="mt-2">{props.description}</p>
      </div>
    </>
  );
};

export const SnippetContent: FunctionComponent = () => {
  const { slug } = useParams();
  const section: SnippetSection | undefined = snippetData.find((s) => s.slug === slug);
  if (!section) return null;

  return (
    <div className="max-w-4xl">
      <h1 className="mb-5 flex items-center gap-4 text-3xl">{section.heading}</h1>
      {section.description && <div className="prose mb-8 max-w-full">{section.description}</div>}

      <div className="space-y-10 pb-10">
        {section.subSections.map((section) => (
          <div key={section.heading}>
            <h2 className="mb-4 text-2xl" id={getAnchorFromHeader(section.heading)}>
              {section.heading}
            </h2>
            <div className="prose max-w-full">{section.content}</div>
          </div>
        ))}
      </div>
    </div>
  );
};
