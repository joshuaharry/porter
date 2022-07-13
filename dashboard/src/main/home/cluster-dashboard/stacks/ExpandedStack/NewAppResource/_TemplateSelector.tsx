import React, { useEffect, useState } from "react";
import api from "shared/api";
import { PorterTemplate } from "shared/types";
import semver from "semver";
import Loading from "components/Loading";
import Placeholder from "components/Placeholder";
import { Card } from "../../launch/components/styles";
import DynamicLink from "components/DynamicLink";
import { VersionSelector } from "../../launch/components/VersionSelector";

const TemplateSelector = () => {
  const [templates, setTemplates] = useState<PorterTemplate[]>([]);
  const [selectedVersion, setSelectedVersion] = useState<{
    [template_name: string]: string;
  }>({});

  const [isLoading, setIsLoading] = useState(true);
  const [hasError, setHasError] = useState(false);

  const getTemplates = async () => {
    try {
      const res = await api.getTemplates<PorterTemplate[]>(
        "<token>",
        {
          repo_url: process.env.APPLICATION_CHART_REPO_URL,
        },
        {}
      );
      let sortedVersionData = res.data
        .map((template: PorterTemplate) => {
          let versions = template.versions.reverse();

          versions = template.versions.sort(semver.rcompare);

          return {
            ...template,
            versions,
            currentVersion: versions[0],
          };
        })
        .sort((a, b) => {
          if (a.name < b.name) {
            return -1;
          }
          if (a.name > b.name) {
            return 1;
          }
          return 0;
        });

      return sortedVersionData;
    } catch (err) {
      throw err;
    }
  };

  useEffect(() => {
    let isSubscribed = true;
    setIsLoading(true);
    getTemplates()
      .then((porterTemplates) => {
        const latestVersions = porterTemplates.reduce((acc, template) => {
          return {
            ...acc,
            [template.name]: template.versions[0],
          };
        }, {} as Record<string, string>);

        if (isSubscribed) {
          setTemplates(porterTemplates);
          setSelectedVersion(latestVersions);
        }
      })
      .catch(() => {
        if (isSubscribed) {
          setHasError(true);
        }
      })
      .finally(() => {
        if (isSubscribed) {
          setIsLoading(false);
        }
      });

    return () => {
      isSubscribed = false;
    };
  }, []);

  if (isLoading) {
    return <Loading />;
  }

  if (hasError) {
    return (
      <Placeholder>
        <div>
          <h2>Unexpected error</h2>
          <p>
            We had an error retrieving the available templates, please try
            again.
          </p>
        </div>
      </Placeholder>
    );
  }

  return (
    <>
      <h2>Select the template</h2>
      <Card.Grid>
        {templates.map((template) => {
          return (
            <Card.Wrapper key={template.name}>
              <Card.Title>
                New {template.name} with version:
                <VersionSelector
                  value={selectedVersion[template.name]}
                  options={template.versions}
                  onChange={(newVersion) => {
                    setSelectedVersion((prev) => ({
                      ...prev,
                      [template.name]: newVersion,
                    }));
                  }}
                />
              </Card.Title>
              <Card.Actions>
                <Card.ActionButton
                  as={DynamicLink}
                  to={`settings/${template.name}/${
                    selectedVersion[template.name]
                  }`}
                >
                  <i className="material-icons-outlined">arrow_forward</i>
                </Card.ActionButton>
              </Card.Actions>
            </Card.Wrapper>
          );
        })}
      </Card.Grid>
    </>
  );
};

export default TemplateSelector;
