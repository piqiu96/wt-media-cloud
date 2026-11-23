import { legacyOperationURL, operationURL } from '../publishing-design/routes.js'
location.replace(legacyOperationURL(new URL(location.href)) || operationURL('personal'))
