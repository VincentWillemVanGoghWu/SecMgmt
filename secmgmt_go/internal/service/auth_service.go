package service

import (
	"errors"
	"sort"

	"secmgmt_go/internal/config"
	"secmgmt_go/internal/domain/dto"
	"secmgmt_go/internal/domain/entity"
	"secmgmt_go/internal/repository"
	"secmgmt_go/internal/util"

	"gorm.io/gorm"
)

var ErrInvalidCredentials = errors.New("username or password is incorrect")

type AuthService struct {
	cfg  *config.Config
	repo *repository.Repository
}

func NewAuthService(cfg *config.Config, repo *repository.Repository) *AuthService {
	return &AuthService{cfg: cfg, repo: repo}
}

func (s *AuthService) Login(payload dto.LoginRequest) (*dto.LoginTokenData, error) {
	user, err := s.repo.FindUserByUsername(payload.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if user.Status != "enabled" || !util.CheckPassword(payload.Password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	accessToken, expiresIn, err := util.GenerateToken(s.cfg.JWTSecretKey, user.ID, user.Username, s.cfg.JWTExpireMinutes)
	if err != nil {
		return nil, err
	}

	return &dto.LoginTokenData{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	}, nil
}

func (s *AuthService) GetMe(userID uint) (*dto.MeData, error) {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	roles, err := s.repo.ListRolesByUserID(userID)
	if err != nil {
		return nil, err
	}
	menus, err := s.repo.ListMenusByUserID(userID)
	if err != nil {
		return nil, err
	}
	permissions, err := s.repo.ListPermissionsByUserID(userID)
	if err != nil {
		return nil, err
	}

	roleInfos := make([]dto.RoleInfo, 0, len(roles))
	dataScopes := make([]dto.DataScopeInfo, 0, len(roles))
	for _, role := range roles {
		roleInfos = append(roleInfos, dto.RoleInfo{
			ID:       role.ID,
			RoleCode: role.RoleCode,
			RoleName: role.RoleName,
			Status:   role.Status,
		})

		var scopeValue *string
		if role.DataScopeValue != "" {
			value := role.DataScopeValue
			scopeValue = &value
		}
		dataScopes = append(dataScopes, dto.DataScopeInfo{
			RoleCode:   role.RoleCode,
			ScopeType:  role.DataScopeType,
			ScopeValue: scopeValue,
		})
	}

	buttonPermissions := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if permission.Code != "" {
			buttonPermissions = append(buttonPermissions, permission.Code)
		}
	}
	sort.Strings(buttonPermissions)

	return &dto.MeData{
		User: dto.AuthUser{
			ID:       user.ID,
			Username: user.Username,
			RealName: user.RealName,
			Status:   user.Status,
		},
		Roles:             roleInfos,
		Menus:             buildMenuTree(menus),
		ButtonPermissions: buttonPermissions,
		DataScopes:        dataScopes,
	}, nil
}

func (s *AuthService) GetMenus(userID uint) ([]dto.MenuItem, error) {
	menus, err := s.repo.ListMenusByUserID(userID)
	if err != nil {
		return nil, err
	}
	return buildMenuTree(menus), nil
}

func buildMenuTree(menus []entity.Menu) []dto.MenuItem {
	byParent := make(map[uint][]entity.Menu)
	var roots []entity.Menu

	for _, menu := range menus {
		if menu.ParentID == nil {
			roots = append(roots, menu)
			continue
		}
		byParent[*menu.ParentID] = append(byParent[*menu.ParentID], menu)
	}

	var walk func(entity.Menu) dto.MenuItem
	walk = func(menu entity.Menu) dto.MenuItem {
		item := dto.MenuItem{
			ID:        menu.ID,
			Key:       menu.Code,
			Label:     menu.Name,
			Icon:      menu.Icon,
			RouteName: menu.RouteName,
			Path:      menu.RoutePath,
		}
		children := byParent[menu.ID]
		if len(children) > 0 {
			item.Children = make([]dto.MenuItem, 0, len(children))
			for _, child := range children {
				item.Children = append(item.Children, walk(child))
			}
		}
		return item
	}

	tree := make([]dto.MenuItem, 0, len(roots))
	for _, root := range roots {
		tree = append(tree, walk(root))
	}
	return tree
}
