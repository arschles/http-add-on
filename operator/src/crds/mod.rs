use kube::{
    api::{PatchParams, PatchStrategy},
    Api,
    Error,
    Client,
};

use serde::{Serialize, Deserialize};
use k8s_openapi::apiextensions_apiserver::pkg::apis::apiextensions::v1::{CustomResourceDefinition};
use kube_derive::{CustomResource};


#[derive(CustomResource, Serialize, Deserialize, Clone, Debug)]
#[kube(group = "keda.sh", version = "v1", kind = "ScaledHTTPApp", namespaced)]
pub struct AppSpec {
    pub name: String,
    pub image: String,
    pub port: u32,
}


pub async fn create_crd(client: &Client, _ns: String) -> Result<CustomResourceDefinition, Error> {
    // let static_ns: &str = &ns;
    // let apps: Api<App> = Api::namespaced(client.clone(), static_ns);
   
    let crds: Api<CustomResourceDefinition> = Api::all(client.clone());
    // TODO: can I get a default one of these?
    let patch_params = PatchParams{
        dry_run: false,
        patch_strategy: PatchStrategy::Apply,
        force: false,
        field_manager: Some("keda-http-operator".into()),
    };
    let yaml = serde_yaml::to_vec(&ScaledHTTPApp::crd()).map_err(|err| {
        Error::DynamicResource(format!("Serde error converting app to YAML ({:?})", err))
    })?;
    crds.patch("scaledhttpapps.keda.sh", &patch_params, yaml).await
}
