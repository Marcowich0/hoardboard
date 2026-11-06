import { Link } from 'react-router'
import Page from './Page.tsx'

export default function NotFound() {
  return (
    <Page title="Page not found">
      <Link to="/" className="text-focus underline underline-offset-2">
        Back to the overview
      </Link>
    </Page>
  )
}
