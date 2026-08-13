import useDocusaurusContext from '@docusaurus/useDocusaurusContext';

export default function OrganizationName() {
  const {siteConfig} = useDocusaurusContext();
  return <>{String(siteConfig.customFields?.organizationName ?? '')}</>;
}
