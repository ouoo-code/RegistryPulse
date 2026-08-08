INSERT INTO registry_categories (id, slug, name, description, enabled, sort_order) VALUES
('dockerhub','dockerhub','Docker Hub','Docker 官方镜像仓库',true,10),
('ghcr','ghcr','GitHub Container Registry','GitHub OCI 镜像仓库',true,20),
('quay','quay','Quay','Red Hat Quay 镜像仓库',true,30),
('mcr','mcr','Microsoft Container Registry','Microsoft 容器镜像仓库',true,40),
('k8s','k8s','Kubernetes Registry','Kubernetes 相关镜像仓库',true,50),
('gcr','gcr','Google Container Registry','Google 容器镜像仓库',true,60),
('elastic','elastic','Elastic Container Registry','Elastic 官方镜像仓库',true,70),
('nvcr','nvcr','NVIDIA Container Registry','NVIDIA 容器镜像仓库',true,80)
ON CONFLICT (id) DO NOTHING;
