package store

import (
	"context"
	"database/sql"
)

const envColumns = `id,name,source_type,deploy_type,repo_url,git_ref,git_token,
	registry_image,registry_username,registry_password,dockerfile_path,build_context,
	image_name,run_ports,run_env,run_volumes,run_networks,restart_policy,compose_path,
	redeploy_schedule,volume_sweep,setup_script,cleanup_script,prune_images,health_type,
	health_target,proxy_host,proxy_port,listen_port,clear_site_data,entrypoint,command,
	entrypoint_script,dockerfile_content,compose_override,auto_deploy,webhook_token,status,created_at,updated_at`

func (s *Store) scanEnv(sc interface{ Scan(...any) error }) (*Environment, error) {
	var e Environment
	var prune, autoDeploy int
	var created, updated string
	err := sc.Scan(&e.ID, &e.Name, &e.SourceType, &e.DeployType, &e.RepoURL, &e.GitRef, &e.GitToken,
		&e.RegistryImage, &e.RegistryUsername, &e.RegistryPassword, &e.DockerfilePath, &e.BuildContext,
		&e.ImageName, &e.RunPorts, &e.RunEnv, &e.RunVolumes, &e.RunNetworks, &e.RestartPolicy, &e.ComposePath,
		&e.RedeploySchedule, &e.VolumeSweep, &e.SetupScript, &e.CleanupScript, &prune, &e.HealthType,
		&e.HealthTarget, &e.ProxyHost, &e.ProxyPort, &e.ListenPort, &e.ClearSiteData,
		&e.Entrypoint, &e.Command, &e.EntrypointScript, &e.DockerfileContent, &e.ComposeOverride,
		&autoDeploy, &e.WebhookToken, &e.Status, &created, &updated)
	if err != nil {
		return nil, err
	}
	e.PruneImages = prune != 0
	e.AutoDeploy = autoDeploy != 0
	e.CreatedAt = parseTime(created)
	e.UpdatedAt = parseTime(updated)
	// decrypt credentials at rest
	if e.GitToken, err = s.box.Decrypt(e.GitToken); err != nil {
		return nil, err
	}
	if e.RegistryPassword, err = s.box.Decrypt(e.RegistryPassword); err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *Store) ListEnvironments(ctx context.Context) ([]*Environment, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+envColumns+` FROM environments ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Environment
	for rows.Next() {
		e, err := s.scanEnv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) GetEnvironment(ctx context.Context, id int64) (*Environment, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+envColumns+` FROM environments WHERE id=?`, id)
	e, err := s.scanEnv(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

// EnvByProxyHost returns the env whose proxy_host matches host (case-insensitive),
// or nil. Only envs with a proxy port configured are considered.
func (s *Store) EnvByProxyHost(ctx context.Context, host string) (*Environment, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+envColumns+` FROM environments
		WHERE proxy_host<>'' AND proxy_port<>'' AND lower(proxy_host)=lower(?)`, host)
	e, err := s.scanEnv(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

// EnvByName returns the env with the given name, or nil.
func (s *Store) EnvByName(ctx context.Context, name string) (*Environment, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+envColumns+` FROM environments WHERE name=?`, name)
	e, err := s.scanEnv(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

// encCreds returns the encrypted forms of the two sensitive fields without
// mutating the caller's struct.
func (s *Store) encCreds(e *Environment) (gitTok, regPw string, err error) {
	if gitTok, err = s.box.Encrypt(e.GitToken); err != nil {
		return
	}
	regPw, err = s.box.Encrypt(e.RegistryPassword)
	return
}

func (s *Store) CreateEnvironment(ctx context.Context, e *Environment) error {
	now := nowStr()
	gitTok, regPw, err := s.encCreds(e)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO environments
		(name,source_type,deploy_type,repo_url,git_ref,git_token,registry_image,registry_username,
		 registry_password,dockerfile_path,build_context,image_name,run_ports,run_env,run_volumes,
		 run_networks,restart_policy,compose_path,redeploy_schedule,volume_sweep,setup_script,
		 cleanup_script,prune_images,health_type,health_target,proxy_host,proxy_port,listen_port,clear_site_data,
		 entrypoint,command,entrypoint_script,dockerfile_content,compose_override,auto_deploy,webhook_token,status,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.Name, e.SourceType, e.DeployType, e.RepoURL, e.GitRef, gitTok, e.RegistryImage,
		e.RegistryUsername, regPw, e.DockerfilePath, e.BuildContext, e.ImageName,
		e.RunPorts, e.RunEnv, e.RunVolumes, e.RunNetworks, e.RestartPolicy, e.ComposePath,
		e.RedeploySchedule, e.VolumeSweep, e.SetupScript, e.CleanupScript, boolToInt(e.PruneImages),
		firstNonEmpty(e.HealthType, "none"), e.HealthTarget, e.ProxyHost, e.ProxyPort, e.ListenPort, e.ClearSiteData,
		e.Entrypoint, e.Command, e.EntrypointScript, e.DockerfileContent, e.ComposeOverride, boolToInt(e.AutoDeploy),
		e.WebhookToken, firstNonEmpty(e.Status, "idle"), now, now)
	if err != nil {
		return err
	}
	e.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) UpdateEnvironment(ctx context.Context, e *Environment) error {
	gitTok, regPw, err := s.encCreds(e)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE environments SET
		name=?,source_type=?,deploy_type=?,repo_url=?,git_ref=?,git_token=?,registry_image=?,
		registry_username=?,registry_password=?,dockerfile_path=?,build_context=?,image_name=?,
		run_ports=?,run_env=?,run_volumes=?,run_networks=?,restart_policy=?,compose_path=?,
		redeploy_schedule=?,volume_sweep=?,setup_script=?,cleanup_script=?,prune_images=?,
		health_type=?,health_target=?,proxy_host=?,proxy_port=?,listen_port=?,clear_site_data=?,
		entrypoint=?,command=?,entrypoint_script=?,dockerfile_content=?,compose_override=?,auto_deploy=?,webhook_token=?,status=?,updated_at=? WHERE id=?`,
		e.Name, e.SourceType, e.DeployType, e.RepoURL, e.GitRef, gitTok, e.RegistryImage,
		e.RegistryUsername, regPw, e.DockerfilePath, e.BuildContext, e.ImageName,
		e.RunPorts, e.RunEnv, e.RunVolumes, e.RunNetworks, e.RestartPolicy, e.ComposePath,
		e.RedeploySchedule, e.VolumeSweep, e.SetupScript, e.CleanupScript, boolToInt(e.PruneImages),
		firstNonEmpty(e.HealthType, "none"), e.HealthTarget, e.ProxyHost, e.ProxyPort, e.ListenPort, e.ClearSiteData,
		e.Entrypoint, e.Command, e.EntrypointScript, e.DockerfileContent, e.ComposeOverride, boolToInt(e.AutoDeploy), e.WebhookToken, e.Status, nowStr(), e.ID)
	return err
}

func (s *Store) SetEnvironmentStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE environments SET status=?, updated_at=? WHERE id=?`,
		status, nowStr(), id)
	return err
}

func (s *Store) DeleteEnvironment(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM environments WHERE id=?`, id)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
