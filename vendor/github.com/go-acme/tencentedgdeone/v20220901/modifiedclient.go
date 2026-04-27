// Copyright (c) 2017-2025 Tencent. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v20220901

import (
    "context"
    "errors"
    "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
    tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
    "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

const APIVersion = "2022-09-01"

type Client struct {
    common.Client
}

// Deprecated
func NewClientWithSecretId(secretId, secretKey, region string) (client *Client, err error) {
    cpf := profile.NewClientProfile()
    client = &Client{}
    client.Init(region).WithSecretId(secretId, secretKey).WithProfile(cpf)
    return
}

func NewClient(credential common.CredentialIface, region string, clientProfile *profile.ClientProfile) (client *Client, err error) {
    client = &Client{}
    client.Init(region).
        WithCredential(credential).
        WithProfile(clientProfile)
    return
}


func NewBindSecurityTemplateToEntityRequest() (request *BindSecurityTemplateToEntityRequest) {
    request = &BindSecurityTemplateToEntityRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "BindSecurityTemplateToEntity")
    
    
    return
}

func NewBindSecurityTemplateToEntityResponse() (response *BindSecurityTemplateToEntityResponse) {
    response = &BindSecurityTemplateToEntityResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// BindSecurityTemplateToEntity
// 操作安全策略模板，支持将域名绑定或换绑到指定的策略模板，或者从指定的策略模板解绑。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNSUPPORTEDOPERATION = "UnsupportedOperation"
func BindSecurityTemplateToEntity(c *Client, request *BindSecurityTemplateToEntityRequest) (response *BindSecurityTemplateToEntityResponse, err error) {
    return BindSecurityTemplateToEntityWithContext(context.Background(), c, request)
}

// BindSecurityTemplateToEntity
// 操作安全策略模板，支持将域名绑定或换绑到指定的策略模板，或者从指定的策略模板解绑。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNSUPPORTEDOPERATION = "UnsupportedOperation"
func BindSecurityTemplateToEntityWithContext(ctx context.Context, c *Client, request *BindSecurityTemplateToEntityRequest) (response *BindSecurityTemplateToEntityResponse, err error) {
    if request == nil {
        request = NewBindSecurityTemplateToEntityRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "BindSecurityTemplateToEntity")
    
    if c.GetCredential() == nil {
        return nil, errors.New("BindSecurityTemplateToEntity require credential")
    }

    request.SetContext(ctx)
    
    response = NewBindSecurityTemplateToEntityResponse()
    err = c.Send(request, response)
    return
}

func NewBindSharedCNAMERequest() (request *BindSharedCNAMERequest) {
    request = &BindSharedCNAMERequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "BindSharedCNAME")
    
    
    return
}

func NewBindSharedCNAMEResponse() (response *BindSharedCNAMEResponse) {
    response = &BindSharedCNAMEResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// BindSharedCNAME
// 用于加速域名绑定或解绑共享 CNAME，该功能白名单内测中。
//
// 可能返回的错误码:
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
func BindSharedCNAME(c *Client, request *BindSharedCNAMERequest) (response *BindSharedCNAMEResponse, err error) {
    return BindSharedCNAMEWithContext(context.Background(), c, request)
}

// BindSharedCNAME
// 用于加速域名绑定或解绑共享 CNAME，该功能白名单内测中。
//
// 可能返回的错误码:
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
func BindSharedCNAMEWithContext(ctx context.Context, c *Client, request *BindSharedCNAMERequest) (response *BindSharedCNAMEResponse, err error) {
    if request == nil {
        request = NewBindSharedCNAMERequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "BindSharedCNAME")
    
    if c.GetCredential() == nil {
        return nil, errors.New("BindSharedCNAME require credential")
    }

    request.SetContext(ctx)
    
    response = NewBindSharedCNAMEResponse()
    err = c.Send(request, response)
    return
}

func NewBindZoneToPlanRequest() (request *BindZoneToPlanRequest) {
    request = &BindZoneToPlanRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "BindZoneToPlan")
    
    
    return
}

func NewBindZoneToPlanResponse() (response *BindZoneToPlanResponse) {
    response = &BindZoneToPlanResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// BindZoneToPlan
// 将未绑定套餐的站点绑定到已有套餐
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  INVALIDPARAMETER_ZONEHASBEENBOUND = "InvalidParameter.ZoneHasBeenBound"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
func BindZoneToPlan(c *Client, request *BindZoneToPlanRequest) (response *BindZoneToPlanResponse, err error) {
    return BindZoneToPlanWithContext(context.Background(), c, request)
}

// BindZoneToPlan
// 将未绑定套餐的站点绑定到已有套餐
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  INVALIDPARAMETER_ZONEHASBEENBOUND = "InvalidParameter.ZoneHasBeenBound"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
func BindZoneToPlanWithContext(ctx context.Context, c *Client, request *BindZoneToPlanRequest) (response *BindZoneToPlanResponse, err error) {
    if request == nil {
        request = NewBindZoneToPlanRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "BindZoneToPlan")
    
    if c.GetCredential() == nil {
        return nil, errors.New("BindZoneToPlan require credential")
    }

    request.SetContext(ctx)
    
    response = NewBindZoneToPlanResponse()
    err = c.Send(request, response)
    return
}

func NewCheckCnameStatusRequest() (request *CheckCnameStatusRequest) {
    request = &CheckCnameStatusRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CheckCnameStatus")
    
    
    return
}

func NewCheckCnameStatusResponse() (response *CheckCnameStatusResponse) {
    response = &CheckCnameStatusResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CheckCnameStatus
// 校验域名 CNAME 状态
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func CheckCnameStatus(c *Client, request *CheckCnameStatusRequest) (response *CheckCnameStatusResponse, err error) {
    return CheckCnameStatusWithContext(context.Background(), c, request)
}

// CheckCnameStatus
// 校验域名 CNAME 状态
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func CheckCnameStatusWithContext(ctx context.Context, c *Client, request *CheckCnameStatusRequest) (response *CheckCnameStatusResponse, err error) {
    if request == nil {
        request = NewCheckCnameStatusRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CheckCnameStatus")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CheckCnameStatus require credential")
    }

    request.SetContext(ctx)
    
    response = NewCheckCnameStatusResponse()
    err = c.Send(request, response)
    return
}

func NewConfirmMultiPathGatewayOriginACLRequest() (request *ConfirmMultiPathGatewayOriginACLRequest) {
    request = &ConfirmMultiPathGatewayOriginACLRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ConfirmMultiPathGatewayOriginACL")
    
    
    return
}

func NewConfirmMultiPathGatewayOriginACLResponse() (response *ConfirmMultiPathGatewayOriginACLResponse) {
    response = &ConfirmMultiPathGatewayOriginACLResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ConfirmMultiPathGatewayOriginACL
// 本接口用于多通道安全加速网关回源 IP 网段发生变更时，确认已将最新回源 IP 网段更新至源站防火墙。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ConfirmMultiPathGatewayOriginACL(c *Client, request *ConfirmMultiPathGatewayOriginACLRequest) (response *ConfirmMultiPathGatewayOriginACLResponse, err error) {
    return ConfirmMultiPathGatewayOriginACLWithContext(context.Background(), c, request)
}

// ConfirmMultiPathGatewayOriginACL
// 本接口用于多通道安全加速网关回源 IP 网段发生变更时，确认已将最新回源 IP 网段更新至源站防火墙。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ConfirmMultiPathGatewayOriginACLWithContext(ctx context.Context, c *Client, request *ConfirmMultiPathGatewayOriginACLRequest) (response *ConfirmMultiPathGatewayOriginACLResponse, err error) {
    if request == nil {
        request = NewConfirmMultiPathGatewayOriginACLRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ConfirmMultiPathGatewayOriginACL")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ConfirmMultiPathGatewayOriginACL require credential")
    }

    request.SetContext(ctx)
    
    response = NewConfirmMultiPathGatewayOriginACLResponse()
    err = c.Send(request, response)
    return
}

func NewConfirmOriginACLUpdateRequest() (request *ConfirmOriginACLUpdateRequest) {
    request = &ConfirmOriginACLUpdateRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ConfirmOriginACLUpdate")
    
    
    return
}

func NewConfirmOriginACLUpdateResponse() (response *ConfirmOriginACLUpdateResponse) {
    response = &ConfirmOriginACLUpdateResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ConfirmOriginACLUpdate
// 本接口用于回源 IP 网段发生变更时，确认已将最新回源 IP 网段更新至源站防火墙。确认已更新至最新的回源 IP 网段后，相关变更通知将会停止推送。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_LATESTVERSIONNOW = "OperationDenied.LatestVersionNow"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func ConfirmOriginACLUpdate(c *Client, request *ConfirmOriginACLUpdateRequest) (response *ConfirmOriginACLUpdateResponse, err error) {
    return ConfirmOriginACLUpdateWithContext(context.Background(), c, request)
}

// ConfirmOriginACLUpdate
// 本接口用于回源 IP 网段发生变更时，确认已将最新回源 IP 网段更新至源站防火墙。确认已更新至最新的回源 IP 网段后，相关变更通知将会停止推送。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_LATESTVERSIONNOW = "OperationDenied.LatestVersionNow"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func ConfirmOriginACLUpdateWithContext(ctx context.Context, c *Client, request *ConfirmOriginACLUpdateRequest) (response *ConfirmOriginACLUpdateResponse, err error) {
    if request == nil {
        request = NewConfirmOriginACLUpdateRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ConfirmOriginACLUpdate")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ConfirmOriginACLUpdate require credential")
    }

    request.SetContext(ctx)
    
    response = NewConfirmOriginACLUpdateResponse()
    err = c.Send(request, response)
    return
}

func NewCreateAccelerationDomainRequest() (request *CreateAccelerationDomainRequest) {
    request = &CreateAccelerationDomainRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateAccelerationDomain")
    
    
    return
}

func NewCreateAccelerationDomainResponse() (response *CreateAccelerationDomainResponse) {
    response = &CreateAccelerationDomainResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateAccelerationDomain
// 在创建完站点之后，您可以通过本接口创建加速域名。 
//
// 
//
// CNAME 模式接入时，若您未完成站点归属权校验，本接口将为您返回域名归属权验证信息，您可以单独对域名进行归属权验证，详情参考 [站点/域名归属权验证](https://cloud.tencent.com/document/product/1552/70789)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INVALIDPARAMETER_CONFLICTHOSTORIGIN = "InvalidParameter.ConflictHostOrigin"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_INVALIDACCELERATETYPE = "InvalidParameter.InvalidAccelerateType"
//  INVALIDPARAMETER_INVALIDAWSPRIVATEACCESS = "InvalidParameter.InvalidAwsPrivateAccess"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDCLIENTIPORIGIN = "InvalidParameter.InvalidClientIpOrigin"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_INVALIDPRIVATEACCESSPARAMS = "InvalidParameter.InvalidPrivateAccessParams"
//  INVALIDPARAMETER_INVALIDPRIVATEACCESSSWITCH = "InvalidParameter.InvalidPrivateAccessSwitch"
//  INVALIDPARAMETER_INVALIDQUICBILLING = "InvalidParameter.InvalidQuicBilling"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGCLIENTIP = "InvalidParameter.InvalidStandardDebugClientIp"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_SPACENOTBINDORIGIN = "InvalidParameter.SpaceNotBindOrigin"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETERVALUE_CONFLICTRECORD = "InvalidParameterValue.ConflictRecord"
//  INVALIDPARAMETERVALUE_CONFLICTWITHDNSSEC = "InvalidParameterValue.ConflictWithDNSSEC"
//  INVALIDPARAMETERVALUE_CONFLICTWITHDOMAIN = "InvalidParameterValue.ConflictWithDomain"
//  INVALIDPARAMETERVALUE_CONFLICTWITHNSRECORD = "InvalidParameterValue.ConflictWithNSRecord"
//  INVALIDPARAMETERVALUE_CONTENTSAMEASNAME = "InvalidParameterValue.ContentSameAsName"
//  INVALIDPARAMETERVALUE_DOMAINNOTMATCHZONE = "InvalidParameterValue.DomainNotMatchZone"
//  INVALIDPARAMETERVALUE_INVALIDDNSCONTENT = "InvalidParameterValue.InvalidDNSContent"
//  INVALIDPARAMETERVALUE_INVALIDDNSNAME = "InvalidParameterValue.InvalidDNSName"
//  INVALIDPARAMETERVALUE_INVALIDDOMAINNAME = "InvalidParameterValue.InvalidDomainName"
//  INVALIDPARAMETERVALUE_INVALIDPROXYORIGIN = "InvalidParameterValue.InvalidProxyOrigin"
//  INVALIDPARAMETERVALUE_ORIGINGROUPNOTEXISTS = "InvalidParameterValue.OriginGroupNotExists"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDDISABLE = "OperationDenied.AccelerateMainlandDisable"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_DOMAINNOICP = "OperationDenied.DomainNoICP"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSEZONEAREA = "OperationDenied.InvalidAdvancedDefenseZoneArea"
//  OPERATIONDENIED_LOADBALANCESTATUSNOTINONLINE = "OperationDenied.LoadBalanceStatusNotInOnline"
//  OPERATIONDENIED_RECORDISFORBIDDEN = "OperationDenied.RecordIsForbidden"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEINUSE_DNSRECORD = "ResourceInUse.DnsRecord"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE_DOMAINALREADYEXISTS = "ResourceUnavailable.DomainAlreadyExists"
//  RESOURCESSOLDOUT_L7LACKOFRESOURCES = "ResourcesSoldOut.L7LackOfResources"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func CreateAccelerationDomain(c *Client, request *CreateAccelerationDomainRequest) (response *CreateAccelerationDomainResponse, err error) {
    return CreateAccelerationDomainWithContext(context.Background(), c, request)
}

// CreateAccelerationDomain
// 在创建完站点之后，您可以通过本接口创建加速域名。 
//
// 
//
// CNAME 模式接入时，若您未完成站点归属权校验，本接口将为您返回域名归属权验证信息，您可以单独对域名进行归属权验证，详情参考 [站点/域名归属权验证](https://cloud.tencent.com/document/product/1552/70789)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INVALIDPARAMETER_CONFLICTHOSTORIGIN = "InvalidParameter.ConflictHostOrigin"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_INVALIDACCELERATETYPE = "InvalidParameter.InvalidAccelerateType"
//  INVALIDPARAMETER_INVALIDAWSPRIVATEACCESS = "InvalidParameter.InvalidAwsPrivateAccess"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDCLIENTIPORIGIN = "InvalidParameter.InvalidClientIpOrigin"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_INVALIDPRIVATEACCESSPARAMS = "InvalidParameter.InvalidPrivateAccessParams"
//  INVALIDPARAMETER_INVALIDPRIVATEACCESSSWITCH = "InvalidParameter.InvalidPrivateAccessSwitch"
//  INVALIDPARAMETER_INVALIDQUICBILLING = "InvalidParameter.InvalidQuicBilling"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGCLIENTIP = "InvalidParameter.InvalidStandardDebugClientIp"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_SPACENOTBINDORIGIN = "InvalidParameter.SpaceNotBindOrigin"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETERVALUE_CONFLICTRECORD = "InvalidParameterValue.ConflictRecord"
//  INVALIDPARAMETERVALUE_CONFLICTWITHDNSSEC = "InvalidParameterValue.ConflictWithDNSSEC"
//  INVALIDPARAMETERVALUE_CONFLICTWITHDOMAIN = "InvalidParameterValue.ConflictWithDomain"
//  INVALIDPARAMETERVALUE_CONFLICTWITHNSRECORD = "InvalidParameterValue.ConflictWithNSRecord"
//  INVALIDPARAMETERVALUE_CONTENTSAMEASNAME = "InvalidParameterValue.ContentSameAsName"
//  INVALIDPARAMETERVALUE_DOMAINNOTMATCHZONE = "InvalidParameterValue.DomainNotMatchZone"
//  INVALIDPARAMETERVALUE_INVALIDDNSCONTENT = "InvalidParameterValue.InvalidDNSContent"
//  INVALIDPARAMETERVALUE_INVALIDDNSNAME = "InvalidParameterValue.InvalidDNSName"
//  INVALIDPARAMETERVALUE_INVALIDDOMAINNAME = "InvalidParameterValue.InvalidDomainName"
//  INVALIDPARAMETERVALUE_INVALIDPROXYORIGIN = "InvalidParameterValue.InvalidProxyOrigin"
//  INVALIDPARAMETERVALUE_ORIGINGROUPNOTEXISTS = "InvalidParameterValue.OriginGroupNotExists"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDDISABLE = "OperationDenied.AccelerateMainlandDisable"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_DOMAINNOICP = "OperationDenied.DomainNoICP"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSEZONEAREA = "OperationDenied.InvalidAdvancedDefenseZoneArea"
//  OPERATIONDENIED_LOADBALANCESTATUSNOTINONLINE = "OperationDenied.LoadBalanceStatusNotInOnline"
//  OPERATIONDENIED_RECORDISFORBIDDEN = "OperationDenied.RecordIsForbidden"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEINUSE_DNSRECORD = "ResourceInUse.DnsRecord"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE_DOMAINALREADYEXISTS = "ResourceUnavailable.DomainAlreadyExists"
//  RESOURCESSOLDOUT_L7LACKOFRESOURCES = "ResourcesSoldOut.L7LackOfResources"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func CreateAccelerationDomainWithContext(ctx context.Context, c *Client, request *CreateAccelerationDomainRequest) (response *CreateAccelerationDomainResponse, err error) {
    if request == nil {
        request = NewCreateAccelerationDomainRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateAccelerationDomain")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateAccelerationDomain require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateAccelerationDomainResponse()
    err = c.Send(request, response)
    return
}

func NewCreateAliasDomainRequest() (request *CreateAliasDomainRequest) {
    request = &CreateAliasDomainRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateAliasDomain")
    
    
    return
}

func NewCreateAliasDomainResponse() (response *CreateAliasDomainResponse) {
    response = &CreateAliasDomainResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateAliasDomain
// 创建别称域名。
//
// 该功能仅企业版套餐支持，并且该功能当前仍在内测中，如需使用，请[联系我们](https://cloud.tencent.com/online-service?from=connect-us)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CERTIFICATENOTFOUND = "FailedOperation.CertificateNotFound"
//  INTERNALERROR = "InternalError"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INVALIDPARAMETER_ALIASDOMAINNOTSUPPORTSMCERT = "InvalidParameter.AliasDomainNotSupportSMCert"
//  INVALIDPARAMETER_CERTNOTMATCHDOMAIN = "InvalidParameter.CertNotMatchDomain"
//  INVALIDPARAMETER_INVALIDHTTPSCERTINFO = "InvalidParameter.InvalidHttpsCertInfo"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_INVALIDALIASDOMAINNAME = "InvalidParameterValue.InvalidAliasDomainName"
//  INVALIDPARAMETERVALUE_INVALIDALIASNAMESUFFIX = "InvalidParameterValue.InvalidAliasNameSuffix"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DOMAINISBLOCKED = "OperationDenied.DomainIsBlocked"
//  OPERATIONDENIED_DOMAINNOICP = "OperationDenied.DomainNoICP"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  RESOURCEINUSE_ALIASNAME = "ResourceInUse.AliasName"
//  RESOURCEINUSE_ALREADYEXISTSASANACCELERATIONDOMAIN = "ResourceInUse.AlreadyExistsAsAnAccelerationDomain"
//  RESOURCEINUSE_DUPLICATENAME = "ResourceInUse.DuplicateName"
//  RESOURCEINUSE_ZONE = "ResourceInUse.Zone"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE_DOMAINALREADYEXISTS = "ResourceUnavailable.DomainAlreadyExists"
//  UNAUTHORIZEDOPERATION = "UnauthorizedOperation"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNSUPPORTEDOPERATION_TARGETNAMEORIGINTYPECOS = "UnsupportedOperation.TargetNameOriginTypeCos"
func CreateAliasDomain(c *Client, request *CreateAliasDomainRequest) (response *CreateAliasDomainResponse, err error) {
    return CreateAliasDomainWithContext(context.Background(), c, request)
}

// CreateAliasDomain
// 创建别称域名。
//
// 该功能仅企业版套餐支持，并且该功能当前仍在内测中，如需使用，请[联系我们](https://cloud.tencent.com/online-service?from=connect-us)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CERTIFICATENOTFOUND = "FailedOperation.CertificateNotFound"
//  INTERNALERROR = "InternalError"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INVALIDPARAMETER_ALIASDOMAINNOTSUPPORTSMCERT = "InvalidParameter.AliasDomainNotSupportSMCert"
//  INVALIDPARAMETER_CERTNOTMATCHDOMAIN = "InvalidParameter.CertNotMatchDomain"
//  INVALIDPARAMETER_INVALIDHTTPSCERTINFO = "InvalidParameter.InvalidHttpsCertInfo"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_INVALIDALIASDOMAINNAME = "InvalidParameterValue.InvalidAliasDomainName"
//  INVALIDPARAMETERVALUE_INVALIDALIASNAMESUFFIX = "InvalidParameterValue.InvalidAliasNameSuffix"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DOMAINISBLOCKED = "OperationDenied.DomainIsBlocked"
//  OPERATIONDENIED_DOMAINNOICP = "OperationDenied.DomainNoICP"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  RESOURCEINUSE_ALIASNAME = "ResourceInUse.AliasName"
//  RESOURCEINUSE_ALREADYEXISTSASANACCELERATIONDOMAIN = "ResourceInUse.AlreadyExistsAsAnAccelerationDomain"
//  RESOURCEINUSE_DUPLICATENAME = "ResourceInUse.DuplicateName"
//  RESOURCEINUSE_ZONE = "ResourceInUse.Zone"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE_DOMAINALREADYEXISTS = "ResourceUnavailable.DomainAlreadyExists"
//  UNAUTHORIZEDOPERATION = "UnauthorizedOperation"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNSUPPORTEDOPERATION_TARGETNAMEORIGINTYPECOS = "UnsupportedOperation.TargetNameOriginTypeCos"
func CreateAliasDomainWithContext(ctx context.Context, c *Client, request *CreateAliasDomainRequest) (response *CreateAliasDomainResponse, err error) {
    if request == nil {
        request = NewCreateAliasDomainRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateAliasDomain")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateAliasDomain require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateAliasDomainResponse()
    err = c.Send(request, response)
    return
}

func NewCreateApplicationProxyRequest() (request *CreateApplicationProxyRequest) {
    request = &CreateApplicationProxyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateApplicationProxy")
    
    
    return
}

func NewCreateApplicationProxyResponse() (response *CreateApplicationProxyResponse) {
    response = &CreateApplicationProxyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateApplicationProxy
// 本接口为旧版，如需调用请尽快迁移至新版 [创建四层代理实例](https://cloud.tencent.com/document/product/1552/103417) 。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  INVALIDPARAMETER_PROXYNAMEDUPLICATING = "InvalidParameter.ProxyNameDuplicating"
//  INVALIDPARAMETER_PROXYNAMENOTMATCHED = "InvalidParameter.ProxyNameNotMatched"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_USERQUOTALIMITED = "LimitExceeded.UserQuotaLimited"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_PLATTYPEIPACCELERATEMAINLANDNOTSUPPORT = "OperationDenied.PlatTypeIPAccelerateMainlandNotSupport"
//  OPERATIONDENIED_ZONENOTACTIVE = "OperationDenied.ZoneNotActive"
//  RESOURCEINSUFFICIENT = "ResourceInsufficient"
//  RESOURCENOTFOUND = "ResourceNotFound"
func CreateApplicationProxy(c *Client, request *CreateApplicationProxyRequest) (response *CreateApplicationProxyResponse, err error) {
    return CreateApplicationProxyWithContext(context.Background(), c, request)
}

// CreateApplicationProxy
// 本接口为旧版，如需调用请尽快迁移至新版 [创建四层代理实例](https://cloud.tencent.com/document/product/1552/103417) 。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  INVALIDPARAMETER_PROXYNAMEDUPLICATING = "InvalidParameter.ProxyNameDuplicating"
//  INVALIDPARAMETER_PROXYNAMENOTMATCHED = "InvalidParameter.ProxyNameNotMatched"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_USERQUOTALIMITED = "LimitExceeded.UserQuotaLimited"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_PLATTYPEIPACCELERATEMAINLANDNOTSUPPORT = "OperationDenied.PlatTypeIPAccelerateMainlandNotSupport"
//  OPERATIONDENIED_ZONENOTACTIVE = "OperationDenied.ZoneNotActive"
//  RESOURCEINSUFFICIENT = "ResourceInsufficient"
//  RESOURCENOTFOUND = "ResourceNotFound"
func CreateApplicationProxyWithContext(ctx context.Context, c *Client, request *CreateApplicationProxyRequest) (response *CreateApplicationProxyResponse, err error) {
    if request == nil {
        request = NewCreateApplicationProxyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateApplicationProxy")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateApplicationProxy require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateApplicationProxyResponse()
    err = c.Send(request, response)
    return
}

func NewCreateApplicationProxyRuleRequest() (request *CreateApplicationProxyRuleRequest) {
    request = &CreateApplicationProxyRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateApplicationProxyRule")
    
    
    return
}

func NewCreateApplicationProxyRuleResponse() (response *CreateApplicationProxyRuleResponse) {
    response = &CreateApplicationProxyRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateApplicationProxyRule
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [创建四层代理转发规则
//
// ](https://cloud.tencent.com/document/product/1552/103416) 。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_INVALIDRULEPROTO = "InvalidParameter.InvalidRuleProto"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  INVALIDPARAMETER_RULEORIGINFORMATERROR = "InvalidParameter.RuleOriginFormatError"
//  INVALIDPARAMETER_RULEORIGINMULTIDOMAIN = "InvalidParameter.RuleOriginMultiDomain"
//  INVALIDPARAMETER_RULEORIGINPORTINTEGER = "InvalidParameter.RuleOriginPortInteger"
//  INVALIDPARAMETER_RULEORIGINVALUEERROR = "InvalidParameter.RuleOriginValueError"
//  INVALIDPARAMETER_RULEPORTDUPLICATING = "InvalidParameter.RulePortDuplicating"
//  INVALIDPARAMETER_RULEPORTGROUP = "InvalidParameter.RulePortGroup"
//  INVALIDPARAMETER_RULEPORTINTEGER = "InvalidParameter.RulePortInteger"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateApplicationProxyRule(c *Client, request *CreateApplicationProxyRuleRequest) (response *CreateApplicationProxyRuleResponse, err error) {
    return CreateApplicationProxyRuleWithContext(context.Background(), c, request)
}

// CreateApplicationProxyRule
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [创建四层代理转发规则
//
// ](https://cloud.tencent.com/document/product/1552/103416) 。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_INVALIDRULEPROTO = "InvalidParameter.InvalidRuleProto"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  INVALIDPARAMETER_RULEORIGINFORMATERROR = "InvalidParameter.RuleOriginFormatError"
//  INVALIDPARAMETER_RULEORIGINMULTIDOMAIN = "InvalidParameter.RuleOriginMultiDomain"
//  INVALIDPARAMETER_RULEORIGINPORTINTEGER = "InvalidParameter.RuleOriginPortInteger"
//  INVALIDPARAMETER_RULEORIGINVALUEERROR = "InvalidParameter.RuleOriginValueError"
//  INVALIDPARAMETER_RULEPORTDUPLICATING = "InvalidParameter.RulePortDuplicating"
//  INVALIDPARAMETER_RULEPORTGROUP = "InvalidParameter.RulePortGroup"
//  INVALIDPARAMETER_RULEPORTINTEGER = "InvalidParameter.RulePortInteger"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateApplicationProxyRuleWithContext(ctx context.Context, c *Client, request *CreateApplicationProxyRuleRequest) (response *CreateApplicationProxyRuleResponse, err error) {
    if request == nil {
        request = NewCreateApplicationProxyRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateApplicationProxyRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateApplicationProxyRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateApplicationProxyRuleResponse()
    err = c.Send(request, response)
    return
}

func NewCreateCLSIndexRequest() (request *CreateCLSIndexRequest) {
    request = &CreateCLSIndexRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateCLSIndex")
    
    
    return
}

func NewCreateCLSIndexResponse() (response *CreateCLSIndexResponse) {
    response = &CreateCLSIndexResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateCLSIndex
// 针对指定实时日志投递任务（task-id），在对应的腾讯云 CLS 日志主题中创建投递日志字段对应的键值索引。如果您在腾讯云 CLS 已经创建索引，本接口将采用合并的方式追加索引。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CREATECLSLOGSETFAILED = "FailedOperation.CreateClsLogSetFailed"
func CreateCLSIndex(c *Client, request *CreateCLSIndexRequest) (response *CreateCLSIndexResponse, err error) {
    return CreateCLSIndexWithContext(context.Background(), c, request)
}

// CreateCLSIndex
// 针对指定实时日志投递任务（task-id），在对应的腾讯云 CLS 日志主题中创建投递日志字段对应的键值索引。如果您在腾讯云 CLS 已经创建索引，本接口将采用合并的方式追加索引。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CREATECLSLOGSETFAILED = "FailedOperation.CreateClsLogSetFailed"
func CreateCLSIndexWithContext(ctx context.Context, c *Client, request *CreateCLSIndexRequest) (response *CreateCLSIndexResponse, err error) {
    if request == nil {
        request = NewCreateCLSIndexRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateCLSIndex")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateCLSIndex require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateCLSIndexResponse()
    err = c.Send(request, response)
    return
}

func NewCreateConfigGroupVersionRequest() (request *CreateConfigGroupVersionRequest) {
    request = &CreateConfigGroupVersionRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateConfigGroupVersion")
    
    
    return
}

func NewCreateConfigGroupVersionResponse() (response *CreateConfigGroupVersionResponse) {
    response = &CreateConfigGroupVersionResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateConfigGroupVersion
// 在版本管理模式下，用于创建指定配置组的新版本。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CONFIGCONDITIONSYNTAXERROR = "FailedOperation.ConfigConditionSyntaxError"
//  FAILEDOPERATION_CONFIGCONDITIONUNKNOWNTARGET = "FailedOperation.ConfigConditionUnknownTarget"
//  FAILEDOPERATION_CONFIGCONDITIONVALUEEMPTYERROR = "FailedOperation.ConfigConditionValueEmptyError"
//  FAILEDOPERATION_CONFIGDUPLICATEKEYERROR = "FailedOperation.ConfigDuplicateKeyError"
//  FAILEDOPERATION_CONFIGFIELDTYPEERROR = "FailedOperation.ConfigFieldTypeError"
//  FAILEDOPERATION_CONFIGFORMATERROR = "FailedOperation.ConfigFormatError"
//  FAILEDOPERATION_CONFIGJSONFORMATERROR = "FailedOperation.ConfigJSONFormatError"
//  FAILEDOPERATION_CONFIGMALFORMEDCONTENT = "FailedOperation.ConfigMalformedContent"
//  FAILEDOPERATION_CONFIGPARAMVALIDATEERRORS = "FailedOperation.ConfigParamValidateErrors"
//  FAILEDOPERATION_CONFIGTIMEPARSINGERROR = "FailedOperation.ConfigTimeParsingError"
//  FAILEDOPERATION_CONFIGUNKNOWNFIELD = "FailedOperation.ConfigUnknownField"
//  FAILEDOPERATION_CONFIGUNSUPPORTEDACTION = "FailedOperation.ConfigUnsupportedAction"
//  FAILEDOPERATION_CONFIGUNSUPPORTEDFORMATVERSION = "FailedOperation.ConfigUnsupportedFormatVersion"
//  FAILEDOPERATION_MISSINGCONFIGCHUNK = "FailedOperation.MissingConfigChunk"
//  FAILEDOPERATION_UNKNOWNCONFIGGROUPTYPE = "FailedOperation.UnknownConfigGroupType"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_DUPLICATERULE = "InvalidParameter.DuplicateRule"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGACTION = "InvalidParameter.InvalidCacheKeyQueryStringAction"
//  INVALIDPARAMETER_INVALIDCONDITIONS = "InvalidParameter.InvalidConditions"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGEFOLLOWORIGIN = "InvalidParameter.InvalidMaxAgeFollowOrigin"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUG = "InvalidParameter.InvalidStandardDebug"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED_RULELIMITEXCEEDED = "LimitExceeded.RuleLimitExceeded"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDIPV6CONFLICT = "OperationDenied.AccelerateMainlandIpv6Conflict"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_NOTINVERSIONCONTROLWHITELIST = "OperationDenied.NotInVersionControlWhiteList"
//  OPERATIONDENIED_OWNERSHIPVERIFICATIONNOTPASSED = "OperationDenied.OwnershipVerificationNotPassed"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  OPERATIONDENIED_WORKMODENOTINVERSIONCONTROL = "OperationDenied.WorkModeNotInVersionControl"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
func CreateConfigGroupVersion(c *Client, request *CreateConfigGroupVersionRequest) (response *CreateConfigGroupVersionResponse, err error) {
    return CreateConfigGroupVersionWithContext(context.Background(), c, request)
}

// CreateConfigGroupVersion
// 在版本管理模式下，用于创建指定配置组的新版本。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CONFIGCONDITIONSYNTAXERROR = "FailedOperation.ConfigConditionSyntaxError"
//  FAILEDOPERATION_CONFIGCONDITIONUNKNOWNTARGET = "FailedOperation.ConfigConditionUnknownTarget"
//  FAILEDOPERATION_CONFIGCONDITIONVALUEEMPTYERROR = "FailedOperation.ConfigConditionValueEmptyError"
//  FAILEDOPERATION_CONFIGDUPLICATEKEYERROR = "FailedOperation.ConfigDuplicateKeyError"
//  FAILEDOPERATION_CONFIGFIELDTYPEERROR = "FailedOperation.ConfigFieldTypeError"
//  FAILEDOPERATION_CONFIGFORMATERROR = "FailedOperation.ConfigFormatError"
//  FAILEDOPERATION_CONFIGJSONFORMATERROR = "FailedOperation.ConfigJSONFormatError"
//  FAILEDOPERATION_CONFIGMALFORMEDCONTENT = "FailedOperation.ConfigMalformedContent"
//  FAILEDOPERATION_CONFIGPARAMVALIDATEERRORS = "FailedOperation.ConfigParamValidateErrors"
//  FAILEDOPERATION_CONFIGTIMEPARSINGERROR = "FailedOperation.ConfigTimeParsingError"
//  FAILEDOPERATION_CONFIGUNKNOWNFIELD = "FailedOperation.ConfigUnknownField"
//  FAILEDOPERATION_CONFIGUNSUPPORTEDACTION = "FailedOperation.ConfigUnsupportedAction"
//  FAILEDOPERATION_CONFIGUNSUPPORTEDFORMATVERSION = "FailedOperation.ConfigUnsupportedFormatVersion"
//  FAILEDOPERATION_MISSINGCONFIGCHUNK = "FailedOperation.MissingConfigChunk"
//  FAILEDOPERATION_UNKNOWNCONFIGGROUPTYPE = "FailedOperation.UnknownConfigGroupType"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_DUPLICATERULE = "InvalidParameter.DuplicateRule"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGACTION = "InvalidParameter.InvalidCacheKeyQueryStringAction"
//  INVALIDPARAMETER_INVALIDCONDITIONS = "InvalidParameter.InvalidConditions"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGEFOLLOWORIGIN = "InvalidParameter.InvalidMaxAgeFollowOrigin"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUG = "InvalidParameter.InvalidStandardDebug"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED_RULELIMITEXCEEDED = "LimitExceeded.RuleLimitExceeded"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDIPV6CONFLICT = "OperationDenied.AccelerateMainlandIpv6Conflict"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_NOTINVERSIONCONTROLWHITELIST = "OperationDenied.NotInVersionControlWhiteList"
//  OPERATIONDENIED_OWNERSHIPVERIFICATIONNOTPASSED = "OperationDenied.OwnershipVerificationNotPassed"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  OPERATIONDENIED_WORKMODENOTINVERSIONCONTROL = "OperationDenied.WorkModeNotInVersionControl"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
func CreateConfigGroupVersionWithContext(ctx context.Context, c *Client, request *CreateConfigGroupVersionRequest) (response *CreateConfigGroupVersionResponse, err error) {
    if request == nil {
        request = NewCreateConfigGroupVersionRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateConfigGroupVersion")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateConfigGroupVersion require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateConfigGroupVersionResponse()
    err = c.Send(request, response)
    return
}

func NewCreateContentIdentifierRequest() (request *CreateContentIdentifierRequest) {
    request = &CreateContentIdentifierRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateContentIdentifier")
    
    
    return
}

func NewCreateContentIdentifierResponse() (response *CreateContentIdentifierResponse) {
    response = &CreateContentIdentifierResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateContentIdentifier
// 创建内容标识符，可以设置描述、标签等信息，同时需要绑定企业版套餐用于统计计费数据；一个内容标识符只能绑定一个计费套餐，一个计费套餐可以绑定多个内容标识符。该功能仅限白名单开放。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CONFIGCONDITIONSYNTAXERROR = "FailedOperation.ConfigConditionSyntaxError"
//  FAILEDOPERATION_CONFIGCONDITIONUNKNOWNTARGET = "FailedOperation.ConfigConditionUnknownTarget"
//  FAILEDOPERATION_CONFIGCONDITIONVALUEEMPTYERROR = "FailedOperation.ConfigConditionValueEmptyError"
//  FAILEDOPERATION_CONFIGDUPLICATEKEYERROR = "FailedOperation.ConfigDuplicateKeyError"
//  FAILEDOPERATION_CONFIGFIELDTYPEERROR = "FailedOperation.ConfigFieldTypeError"
//  FAILEDOPERATION_CONFIGFORMATERROR = "FailedOperation.ConfigFormatError"
//  FAILEDOPERATION_CONFIGJSONFORMATERROR = "FailedOperation.ConfigJSONFormatError"
//  FAILEDOPERATION_CONFIGMALFORMEDCONTENT = "FailedOperation.ConfigMalformedContent"
//  FAILEDOPERATION_CONFIGPARAMVALIDATEERRORS = "FailedOperation.ConfigParamValidateErrors"
//  FAILEDOPERATION_CONFIGTIMEPARSINGERROR = "FailedOperation.ConfigTimeParsingError"
//  FAILEDOPERATION_CONFIGUNKNOWNFIELD = "FailedOperation.ConfigUnknownField"
//  FAILEDOPERATION_CONFIGUNSUPPORTEDACTION = "FailedOperation.ConfigUnsupportedAction"
//  FAILEDOPERATION_CONFIGUNSUPPORTEDFORMATVERSION = "FailedOperation.ConfigUnsupportedFormatVersion"
//  FAILEDOPERATION_MISSINGCONFIGCHUNK = "FailedOperation.MissingConfigChunk"
//  FAILEDOPERATION_UNKNOWNCONFIGGROUPTYPE = "FailedOperation.UnknownConfigGroupType"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_DUPLICATERULE = "InvalidParameter.DuplicateRule"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGACTION = "InvalidParameter.InvalidCacheKeyQueryStringAction"
//  INVALIDPARAMETER_INVALIDCONDITIONS = "InvalidParameter.InvalidConditions"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGEFOLLOWORIGIN = "InvalidParameter.InvalidMaxAgeFollowOrigin"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUG = "InvalidParameter.InvalidStandardDebug"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED_RULELIMITEXCEEDED = "LimitExceeded.RuleLimitExceeded"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDIPV6CONFLICT = "OperationDenied.AccelerateMainlandIpv6Conflict"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_NOTINVERSIONCONTROLWHITELIST = "OperationDenied.NotInVersionControlWhiteList"
//  OPERATIONDENIED_OWNERSHIPVERIFICATIONNOTPASSED = "OperationDenied.OwnershipVerificationNotPassed"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  OPERATIONDENIED_WORKMODENOTINVERSIONCONTROL = "OperationDenied.WorkModeNotInVersionControl"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
func CreateContentIdentifier(c *Client, request *CreateContentIdentifierRequest) (response *CreateContentIdentifierResponse, err error) {
    return CreateContentIdentifierWithContext(context.Background(), c, request)
}

// CreateContentIdentifier
// 创建内容标识符，可以设置描述、标签等信息，同时需要绑定企业版套餐用于统计计费数据；一个内容标识符只能绑定一个计费套餐，一个计费套餐可以绑定多个内容标识符。该功能仅限白名单开放。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CONFIGCONDITIONSYNTAXERROR = "FailedOperation.ConfigConditionSyntaxError"
//  FAILEDOPERATION_CONFIGCONDITIONUNKNOWNTARGET = "FailedOperation.ConfigConditionUnknownTarget"
//  FAILEDOPERATION_CONFIGCONDITIONVALUEEMPTYERROR = "FailedOperation.ConfigConditionValueEmptyError"
//  FAILEDOPERATION_CONFIGDUPLICATEKEYERROR = "FailedOperation.ConfigDuplicateKeyError"
//  FAILEDOPERATION_CONFIGFIELDTYPEERROR = "FailedOperation.ConfigFieldTypeError"
//  FAILEDOPERATION_CONFIGFORMATERROR = "FailedOperation.ConfigFormatError"
//  FAILEDOPERATION_CONFIGJSONFORMATERROR = "FailedOperation.ConfigJSONFormatError"
//  FAILEDOPERATION_CONFIGMALFORMEDCONTENT = "FailedOperation.ConfigMalformedContent"
//  FAILEDOPERATION_CONFIGPARAMVALIDATEERRORS = "FailedOperation.ConfigParamValidateErrors"
//  FAILEDOPERATION_CONFIGTIMEPARSINGERROR = "FailedOperation.ConfigTimeParsingError"
//  FAILEDOPERATION_CONFIGUNKNOWNFIELD = "FailedOperation.ConfigUnknownField"
//  FAILEDOPERATION_CONFIGUNSUPPORTEDACTION = "FailedOperation.ConfigUnsupportedAction"
//  FAILEDOPERATION_CONFIGUNSUPPORTEDFORMATVERSION = "FailedOperation.ConfigUnsupportedFormatVersion"
//  FAILEDOPERATION_MISSINGCONFIGCHUNK = "FailedOperation.MissingConfigChunk"
//  FAILEDOPERATION_UNKNOWNCONFIGGROUPTYPE = "FailedOperation.UnknownConfigGroupType"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_DUPLICATERULE = "InvalidParameter.DuplicateRule"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGACTION = "InvalidParameter.InvalidCacheKeyQueryStringAction"
//  INVALIDPARAMETER_INVALIDCONDITIONS = "InvalidParameter.InvalidConditions"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGEFOLLOWORIGIN = "InvalidParameter.InvalidMaxAgeFollowOrigin"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUG = "InvalidParameter.InvalidStandardDebug"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED_RULELIMITEXCEEDED = "LimitExceeded.RuleLimitExceeded"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDIPV6CONFLICT = "OperationDenied.AccelerateMainlandIpv6Conflict"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_NOTINVERSIONCONTROLWHITELIST = "OperationDenied.NotInVersionControlWhiteList"
//  OPERATIONDENIED_OWNERSHIPVERIFICATIONNOTPASSED = "OperationDenied.OwnershipVerificationNotPassed"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  OPERATIONDENIED_WORKMODENOTINVERSIONCONTROL = "OperationDenied.WorkModeNotInVersionControl"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
func CreateContentIdentifierWithContext(ctx context.Context, c *Client, request *CreateContentIdentifierRequest) (response *CreateContentIdentifierResponse, err error) {
    if request == nil {
        request = NewCreateContentIdentifierRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateContentIdentifier")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateContentIdentifier require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateContentIdentifierResponse()
    err = c.Send(request, response)
    return
}

func NewCreateCustomizeErrorPageRequest() (request *CreateCustomizeErrorPageRequest) {
    request = &CreateCustomizeErrorPageRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateCustomizeErrorPage")
    
    
    return
}

func NewCreateCustomizeErrorPageResponse() (response *CreateCustomizeErrorPageResponse) {
    response = &CreateCustomizeErrorPageResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateCustomizeErrorPage
// 创建自定义错误页面。
//
// 可能返回的错误码:
//  INVALIDPARAMETERVALUE_CONTENTTYPENOTMATCH = "InvalidParameterValue.ContentTypeNotMatch"
//  INVALIDPARAMETERVALUE_PAGENAMEALREADYEXIST = "InvalidParameterValue.PageNameAlreadyExist"
func CreateCustomizeErrorPage(c *Client, request *CreateCustomizeErrorPageRequest) (response *CreateCustomizeErrorPageResponse, err error) {
    return CreateCustomizeErrorPageWithContext(context.Background(), c, request)
}

// CreateCustomizeErrorPage
// 创建自定义错误页面。
//
// 可能返回的错误码:
//  INVALIDPARAMETERVALUE_CONTENTTYPENOTMATCH = "InvalidParameterValue.ContentTypeNotMatch"
//  INVALIDPARAMETERVALUE_PAGENAMEALREADYEXIST = "InvalidParameterValue.PageNameAlreadyExist"
func CreateCustomizeErrorPageWithContext(ctx context.Context, c *Client, request *CreateCustomizeErrorPageRequest) (response *CreateCustomizeErrorPageResponse, err error) {
    if request == nil {
        request = NewCreateCustomizeErrorPageRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateCustomizeErrorPage")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateCustomizeErrorPage require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateCustomizeErrorPageResponse()
    err = c.Send(request, response)
    return
}

func NewCreateDnsRecordRequest() (request *CreateDnsRecordRequest) {
    request = &CreateDnsRecordRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateDnsRecord")
    
    
    return
}

func NewCreateDnsRecordResponse() (response *CreateDnsRecordResponse) {
    response = &CreateDnsRecordResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateDnsRecord
// 在创建完站点后，并且站点为 NS 模式接入时，您可以通过本接口创建 DNS 记录。
//
// 可能返回的错误码:
//  INVALIDPARAMETERVALUE_CONTENTTYPENOTMATCH = "InvalidParameterValue.ContentTypeNotMatch"
//  INVALIDPARAMETERVALUE_PAGENAMEALREADYEXIST = "InvalidParameterValue.PageNameAlreadyExist"
func CreateDnsRecord(c *Client, request *CreateDnsRecordRequest) (response *CreateDnsRecordResponse, err error) {
    return CreateDnsRecordWithContext(context.Background(), c, request)
}

// CreateDnsRecord
// 在创建完站点后，并且站点为 NS 模式接入时，您可以通过本接口创建 DNS 记录。
//
// 可能返回的错误码:
//  INVALIDPARAMETERVALUE_CONTENTTYPENOTMATCH = "InvalidParameterValue.ContentTypeNotMatch"
//  INVALIDPARAMETERVALUE_PAGENAMEALREADYEXIST = "InvalidParameterValue.PageNameAlreadyExist"
func CreateDnsRecordWithContext(ctx context.Context, c *Client, request *CreateDnsRecordRequest) (response *CreateDnsRecordResponse, err error) {
    if request == nil {
        request = NewCreateDnsRecordRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateDnsRecord")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateDnsRecord require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateDnsRecordResponse()
    err = c.Send(request, response)
    return
}

func NewCreateFunctionRequest() (request *CreateFunctionRequest) {
    request = &CreateFunctionRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateFunction")
    
    
    return
}

func NewCreateFunctionResponse() (response *CreateFunctionResponse) {
    response = &CreateFunctionResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateFunction
// 创建并部署边缘函数至 EdgeOne 的边缘节点。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_FUNCTIONDEPLOYING = "FailedOperation.FunctionDeploying"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_BADCONTENT = "InvalidParameter.BadContent"
//  INVALIDPARAMETER_BADFUNCTIONNAME = "InvalidParameter.BadFunctionName"
//  INVALIDPARAMETER_CONTENTEXCEEDSLIMIT = "InvalidParameter.ContentExceedsLimit"
//  INVALIDPARAMETER_FUNCTIONNAMECONFLICT = "InvalidParameter.FunctionNameConflict"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  LIMITEXCEEDED_FUNCTIONLIMITEXCEEDED = "LimitExceeded.FunctionLimitExceeded"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateFunction(c *Client, request *CreateFunctionRequest) (response *CreateFunctionResponse, err error) {
    return CreateFunctionWithContext(context.Background(), c, request)
}

// CreateFunction
// 创建并部署边缘函数至 EdgeOne 的边缘节点。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_FUNCTIONDEPLOYING = "FailedOperation.FunctionDeploying"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_BADCONTENT = "InvalidParameter.BadContent"
//  INVALIDPARAMETER_BADFUNCTIONNAME = "InvalidParameter.BadFunctionName"
//  INVALIDPARAMETER_CONTENTEXCEEDSLIMIT = "InvalidParameter.ContentExceedsLimit"
//  INVALIDPARAMETER_FUNCTIONNAMECONFLICT = "InvalidParameter.FunctionNameConflict"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  LIMITEXCEEDED_FUNCTIONLIMITEXCEEDED = "LimitExceeded.FunctionLimitExceeded"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateFunctionWithContext(ctx context.Context, c *Client, request *CreateFunctionRequest) (response *CreateFunctionResponse, err error) {
    if request == nil {
        request = NewCreateFunctionRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateFunction")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateFunction require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateFunctionResponse()
    err = c.Send(request, response)
    return
}

func NewCreateFunctionRuleRequest() (request *CreateFunctionRuleRequest) {
    request = &CreateFunctionRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateFunctionRule")
    
    
    return
}

func NewCreateFunctionRuleResponse() (response *CreateFunctionRuleResponse) {
    response = &CreateFunctionRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateFunctionRule
// 创建边缘函数的触发规则。支持通过自定义过滤条件来决定是否需要执行函数，当需要执行函数时，提供了多种选择目标函数的方式，包括：直接指定，基于客户端归属地区选择和基于权重选择。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_RULEOPERATIONCONFLICT = "FailedOperation.RuleOperationConflict"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_DUPLICATERULE = "InvalidParameter.DuplicateRule"
//  INVALIDPARAMETER_INVALIDCONDITIONS = "InvalidParameter.InvalidConditions"
//  LIMITEXCEEDED_RULELIMITEXCEEDED = "LimitExceeded.RuleLimitExceeded"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func CreateFunctionRule(c *Client, request *CreateFunctionRuleRequest) (response *CreateFunctionRuleResponse, err error) {
    return CreateFunctionRuleWithContext(context.Background(), c, request)
}

// CreateFunctionRule
// 创建边缘函数的触发规则。支持通过自定义过滤条件来决定是否需要执行函数，当需要执行函数时，提供了多种选择目标函数的方式，包括：直接指定，基于客户端归属地区选择和基于权重选择。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_RULEOPERATIONCONFLICT = "FailedOperation.RuleOperationConflict"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_DUPLICATERULE = "InvalidParameter.DuplicateRule"
//  INVALIDPARAMETER_INVALIDCONDITIONS = "InvalidParameter.InvalidConditions"
//  LIMITEXCEEDED_RULELIMITEXCEEDED = "LimitExceeded.RuleLimitExceeded"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func CreateFunctionRuleWithContext(ctx context.Context, c *Client, request *CreateFunctionRuleRequest) (response *CreateFunctionRuleResponse, err error) {
    if request == nil {
        request = NewCreateFunctionRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateFunctionRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateFunctionRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateFunctionRuleResponse()
    err = c.Send(request, response)
    return
}

func NewCreateJustInTimeTranscodeTemplateRequest() (request *CreateJustInTimeTranscodeTemplateRequest) {
    request = &CreateJustInTimeTranscodeTemplateRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateJustInTimeTranscodeTemplate")
    
    
    return
}

func NewCreateJustInTimeTranscodeTemplateResponse() (response *CreateJustInTimeTranscodeTemplateResponse) {
    response = &CreateJustInTimeTranscodeTemplateResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateJustInTimeTranscodeTemplate
// 即时转码已经提供了预置转码模板，满足大部分的需求。如果有个性化的转码需求，可以通过本接口创建自定义的转码模板，最多可创建100个自定义转码模板。
//
// 为了确保即时转码效果的一致性，避免因 EO 缓存或 M3U8 分片处理过程中的模板变更导致视频输出异常，模板在创建后不可进行修改。
//
// 即时转码详细能力了解：[EdgeOne视频即时处理功能介绍](https://cloud.tencent.com/document/product/1552/111927)。
//
// 可能返回的错误码:
//  FAILEDOPERATION_PRODUCTDISCONTINUED = "FailedOperation.ProductDiscontinued"
//  FAILEDOPERATION_PRODUCTNOTACTIVATED = "FailedOperation.ProductNotActivated"
//  FAILEDOPERATION_TEMPLATEOVERLIMIT = "FailedOperation.TemplateOverLimit"
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_ZONENOTFOUND = "InvalidParameterValue.ZoneNotFound"
//  INVALIDPARAMETERVALUE_ZONEPAUSED = "InvalidParameterValue.ZonePaused"
//  MISSINGPARAMETER = "MissingParameter"
//  UNAUTHORIZEDOPERATION = "UnauthorizedOperation"
func CreateJustInTimeTranscodeTemplate(c *Client, request *CreateJustInTimeTranscodeTemplateRequest) (response *CreateJustInTimeTranscodeTemplateResponse, err error) {
    return CreateJustInTimeTranscodeTemplateWithContext(context.Background(), c, request)
}

// CreateJustInTimeTranscodeTemplate
// 即时转码已经提供了预置转码模板，满足大部分的需求。如果有个性化的转码需求，可以通过本接口创建自定义的转码模板，最多可创建100个自定义转码模板。
//
// 为了确保即时转码效果的一致性，避免因 EO 缓存或 M3U8 分片处理过程中的模板变更导致视频输出异常，模板在创建后不可进行修改。
//
// 即时转码详细能力了解：[EdgeOne视频即时处理功能介绍](https://cloud.tencent.com/document/product/1552/111927)。
//
// 可能返回的错误码:
//  FAILEDOPERATION_PRODUCTDISCONTINUED = "FailedOperation.ProductDiscontinued"
//  FAILEDOPERATION_PRODUCTNOTACTIVATED = "FailedOperation.ProductNotActivated"
//  FAILEDOPERATION_TEMPLATEOVERLIMIT = "FailedOperation.TemplateOverLimit"
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_ZONENOTFOUND = "InvalidParameterValue.ZoneNotFound"
//  INVALIDPARAMETERVALUE_ZONEPAUSED = "InvalidParameterValue.ZonePaused"
//  MISSINGPARAMETER = "MissingParameter"
//  UNAUTHORIZEDOPERATION = "UnauthorizedOperation"
func CreateJustInTimeTranscodeTemplateWithContext(ctx context.Context, c *Client, request *CreateJustInTimeTranscodeTemplateRequest) (response *CreateJustInTimeTranscodeTemplateResponse, err error) {
    if request == nil {
        request = NewCreateJustInTimeTranscodeTemplateRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateJustInTimeTranscodeTemplate")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateJustInTimeTranscodeTemplate require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateJustInTimeTranscodeTemplateResponse()
    err = c.Send(request, response)
    return
}

func NewCreateL4ProxyRequest() (request *CreateL4ProxyRequest) {
    request = &CreateL4ProxyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateL4Proxy")
    
    
    return
}

func NewCreateL4ProxyResponse() (response *CreateL4ProxyResponse) {
    response = &CreateL4ProxyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateL4Proxy
// 用于创建四层代理实例。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_PROXYNAMEDUPLICATING = "InvalidParameter.ProxyNameDuplicating"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDDISABLE = "OperationDenied.AccelerateMainlandDisable"
//  OPERATIONDENIED_ACCELERATEMAINLANDIPV6CONFLICT = "OperationDenied.AccelerateMainlandIpv6Conflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_IPV6ADVANCEDCONFLICT = "OperationDenied.Ipv6AdvancedConflict"
//  OPERATIONDENIED_IPV6STATICIPCONFLICT = "OperationDenied.Ipv6StaticIpConflict"
//  OPERATIONDENIED_L4LACKOFRESOURCES = "OperationDenied.L4LackOfResources"
//  OPERATIONDENIED_MSGIPV6ADVANCEDCONFLICT = "OperationDenied.MsgIpv6AdvancedConflict"
//  OPERATIONDENIED_STATICIPAREACONFLICT = "OperationDenied.StaticIpAreaConflict"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateL4Proxy(c *Client, request *CreateL4ProxyRequest) (response *CreateL4ProxyResponse, err error) {
    return CreateL4ProxyWithContext(context.Background(), c, request)
}

// CreateL4Proxy
// 用于创建四层代理实例。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_PROXYNAMEDUPLICATING = "InvalidParameter.ProxyNameDuplicating"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDDISABLE = "OperationDenied.AccelerateMainlandDisable"
//  OPERATIONDENIED_ACCELERATEMAINLANDIPV6CONFLICT = "OperationDenied.AccelerateMainlandIpv6Conflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_IPV6ADVANCEDCONFLICT = "OperationDenied.Ipv6AdvancedConflict"
//  OPERATIONDENIED_IPV6STATICIPCONFLICT = "OperationDenied.Ipv6StaticIpConflict"
//  OPERATIONDENIED_L4LACKOFRESOURCES = "OperationDenied.L4LackOfResources"
//  OPERATIONDENIED_MSGIPV6ADVANCEDCONFLICT = "OperationDenied.MsgIpv6AdvancedConflict"
//  OPERATIONDENIED_STATICIPAREACONFLICT = "OperationDenied.StaticIpAreaConflict"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateL4ProxyWithContext(ctx context.Context, c *Client, request *CreateL4ProxyRequest) (response *CreateL4ProxyResponse, err error) {
    if request == nil {
        request = NewCreateL4ProxyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateL4Proxy")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateL4Proxy require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateL4ProxyResponse()
    err = c.Send(request, response)
    return
}

func NewCreateL4ProxyRulesRequest() (request *CreateL4ProxyRulesRequest) {
    request = &CreateL4ProxyRulesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateL4ProxyRules")
    
    
    return
}

func NewCreateL4ProxyRulesResponse() (response *CreateL4ProxyRulesResponse) {
    response = &CreateL4ProxyRulesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateL4ProxyRules
// 用于创建四层代理实例规则，支持单条或者批量创建。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_INVALIDORIGINVALUE = "InvalidParameter.InvalidOriginValue"
//  INVALIDPARAMETER_RULEPORTDUPLICATING = "InvalidParameter.RulePortDuplicating"
//  LIMITEXCEEDED_PROXYRULESLIMITEXCEEDED = "LimitExceeded.ProxyRulesLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_L4PORTLACKOFRESOURCES = "OperationDenied.L4PortLackOfResources"
//  RESOURCENOTFOUND = "ResourceNotFound"
func CreateL4ProxyRules(c *Client, request *CreateL4ProxyRulesRequest) (response *CreateL4ProxyRulesResponse, err error) {
    return CreateL4ProxyRulesWithContext(context.Background(), c, request)
}

// CreateL4ProxyRules
// 用于创建四层代理实例规则，支持单条或者批量创建。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_INVALIDORIGINVALUE = "InvalidParameter.InvalidOriginValue"
//  INVALIDPARAMETER_RULEPORTDUPLICATING = "InvalidParameter.RulePortDuplicating"
//  LIMITEXCEEDED_PROXYRULESLIMITEXCEEDED = "LimitExceeded.ProxyRulesLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_L4PORTLACKOFRESOURCES = "OperationDenied.L4PortLackOfResources"
//  RESOURCENOTFOUND = "ResourceNotFound"
func CreateL4ProxyRulesWithContext(ctx context.Context, c *Client, request *CreateL4ProxyRulesRequest) (response *CreateL4ProxyRulesResponse, err error) {
    if request == nil {
        request = NewCreateL4ProxyRulesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateL4ProxyRules")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateL4ProxyRules require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateL4ProxyRulesResponse()
    err = c.Send(request, response)
    return
}

func NewCreateL7AccRulesRequest() (request *CreateL7AccRulesRequest) {
    request = &CreateL7AccRulesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateL7AccRules")
    
    
    return
}

func NewCreateL7AccRulesResponse() (response *CreateL7AccRulesResponse) {
    response = &CreateL7AccRulesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateL7AccRules
// 本接口用于在[规则引擎](https://cloud.tencent.com/document/product/1552/70901)中创建规则，支持批量创建。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACCESSREDIRECTREGEXERROR = "InvalidParameter.AccessRedirectRegexError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONORIGINPRIVATEADDRESS = "InvalidParameter.ErrInvalidActionOriginPrivateAddress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMBADVALUETYPE = "InvalidParameter.ErrInvalidActionParamBadValueType"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITION = "InvalidParameter.ErrInvalidCondition"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONCANNOTONLYCONTAINHOSTWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionCannotOnlyContainHostWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYREGULAR = "InvalidParameter.ErrInvalidConditionValueTooManyRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEZEROLENGTH = "InvalidParameter.ErrInvalidConditionValueZeroLength"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATION = "InvalidParameter.InvalidAuthentication"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPEEXPIRETIME = "InvalidParameter.InvalidAuthenticationTypeExpireTime"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHECONFIGCACHE = "InvalidParameter.InvalidCacheConfigCache"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECT = "InvalidParameter.InvalidUrlRedirect"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_TASKSYSTEMERROR = "InvalidParameter.TaskSystemError"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateL7AccRules(c *Client, request *CreateL7AccRulesRequest) (response *CreateL7AccRulesResponse, err error) {
    return CreateL7AccRulesWithContext(context.Background(), c, request)
}

// CreateL7AccRules
// 本接口用于在[规则引擎](https://cloud.tencent.com/document/product/1552/70901)中创建规则，支持批量创建。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACCESSREDIRECTREGEXERROR = "InvalidParameter.AccessRedirectRegexError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONORIGINPRIVATEADDRESS = "InvalidParameter.ErrInvalidActionOriginPrivateAddress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMBADVALUETYPE = "InvalidParameter.ErrInvalidActionParamBadValueType"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITION = "InvalidParameter.ErrInvalidCondition"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONCANNOTONLYCONTAINHOSTWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionCannotOnlyContainHostWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYREGULAR = "InvalidParameter.ErrInvalidConditionValueTooManyRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEZEROLENGTH = "InvalidParameter.ErrInvalidConditionValueZeroLength"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATION = "InvalidParameter.InvalidAuthentication"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPEEXPIRETIME = "InvalidParameter.InvalidAuthenticationTypeExpireTime"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHECONFIGCACHE = "InvalidParameter.InvalidCacheConfigCache"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECT = "InvalidParameter.InvalidUrlRedirect"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_TASKSYSTEMERROR = "InvalidParameter.TaskSystemError"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateL7AccRulesWithContext(ctx context.Context, c *Client, request *CreateL7AccRulesRequest) (response *CreateL7AccRulesResponse, err error) {
    if request == nil {
        request = NewCreateL7AccRulesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateL7AccRules")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateL7AccRules require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateL7AccRulesResponse()
    err = c.Send(request, response)
    return
}

func NewCreateLoadBalancerRequest() (request *CreateLoadBalancerRequest) {
    request = &CreateLoadBalancerRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateLoadBalancer")
    
    
    return
}

func NewCreateLoadBalancerResponse() (response *CreateLoadBalancerResponse) {
    response = &CreateLoadBalancerResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateLoadBalancer
// 创建负载均衡实例。详情请参考 [快速创建负载均衡实例](https://cloud.tencent.com/document/product/1552/104223)。负载均衡功能内测中，如您需要使用请 [联系我们](https://cloud.tencent.com/online-service)。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_LOADBALANCERBINDORIGINGROUPINVALID = "InvalidParameter.LoadBalancerBindOriginGroupInvalid"
//  INVALIDPARAMETER_LOADBALANCERNAMEREPEATED = "InvalidParameter.LoadBalancerNameRepeated"
//  INVALIDPARAMETER_ORIGINGROUPTYPECANNOTMATCHLBTYPE = "InvalidParameter.OriginGroupTypeCanNotMatchLBType"
//  INVALIDPARAMETER_SOMEORIGINGROUPNOTEXIST = "InvalidParameter.SomeOriginGroupNotExist"
//  LIMITEXCEEDED_LOADBALANCINGCOUNTLIMITEXCEEDED = "LimitExceeded.LoadBalancingCountLimitExceeded"
func CreateLoadBalancer(c *Client, request *CreateLoadBalancerRequest) (response *CreateLoadBalancerResponse, err error) {
    return CreateLoadBalancerWithContext(context.Background(), c, request)
}

// CreateLoadBalancer
// 创建负载均衡实例。详情请参考 [快速创建负载均衡实例](https://cloud.tencent.com/document/product/1552/104223)。负载均衡功能内测中，如您需要使用请 [联系我们](https://cloud.tencent.com/online-service)。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_LOADBALANCERBINDORIGINGROUPINVALID = "InvalidParameter.LoadBalancerBindOriginGroupInvalid"
//  INVALIDPARAMETER_LOADBALANCERNAMEREPEATED = "InvalidParameter.LoadBalancerNameRepeated"
//  INVALIDPARAMETER_ORIGINGROUPTYPECANNOTMATCHLBTYPE = "InvalidParameter.OriginGroupTypeCanNotMatchLBType"
//  INVALIDPARAMETER_SOMEORIGINGROUPNOTEXIST = "InvalidParameter.SomeOriginGroupNotExist"
//  LIMITEXCEEDED_LOADBALANCINGCOUNTLIMITEXCEEDED = "LimitExceeded.LoadBalancingCountLimitExceeded"
func CreateLoadBalancerWithContext(ctx context.Context, c *Client, request *CreateLoadBalancerRequest) (response *CreateLoadBalancerResponse, err error) {
    if request == nil {
        request = NewCreateLoadBalancerRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateLoadBalancer")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateLoadBalancer require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateLoadBalancerResponse()
    err = c.Send(request, response)
    return
}

func NewCreateMultiPathGatewayRequest() (request *CreateMultiPathGatewayRequest) {
    request = &CreateMultiPathGatewayRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateMultiPathGateway")
    
    
    return
}

func NewCreateMultiPathGatewayResponse() (response *CreateMultiPathGatewayResponse) {
    response = &CreateMultiPathGatewayResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateMultiPathGateway
// 通过本接口创建多通道安全加速网关，包括云上网关（腾讯云创建和管理的网关）和自有网关（用户部署的私有网关），需要通过接口 DescribeMultiPathGateway，查询状态为 online 即创建成功。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func CreateMultiPathGateway(c *Client, request *CreateMultiPathGatewayRequest) (response *CreateMultiPathGatewayResponse, err error) {
    return CreateMultiPathGatewayWithContext(context.Background(), c, request)
}

// CreateMultiPathGateway
// 通过本接口创建多通道安全加速网关，包括云上网关（腾讯云创建和管理的网关）和自有网关（用户部署的私有网关），需要通过接口 DescribeMultiPathGateway，查询状态为 online 即创建成功。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func CreateMultiPathGatewayWithContext(ctx context.Context, c *Client, request *CreateMultiPathGatewayRequest) (response *CreateMultiPathGatewayResponse, err error) {
    if request == nil {
        request = NewCreateMultiPathGatewayRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateMultiPathGateway")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateMultiPathGateway require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateMultiPathGatewayResponse()
    err = c.Send(request, response)
    return
}

func NewCreateMultiPathGatewayLineRequest() (request *CreateMultiPathGatewayLineRequest) {
    request = &CreateMultiPathGatewayLineRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateMultiPathGatewayLine")
    
    
    return
}

func NewCreateMultiPathGatewayLineResponse() (response *CreateMultiPathGatewayLineResponse) {
    response = &CreateMultiPathGatewayLineResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateMultiPathGatewayLine
// 通过本接口创建接入多通道安全加速网关的线路。包括 EdgeOne 四层代理线路、自定义线路。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func CreateMultiPathGatewayLine(c *Client, request *CreateMultiPathGatewayLineRequest) (response *CreateMultiPathGatewayLineResponse, err error) {
    return CreateMultiPathGatewayLineWithContext(context.Background(), c, request)
}

// CreateMultiPathGatewayLine
// 通过本接口创建接入多通道安全加速网关的线路。包括 EdgeOne 四层代理线路、自定义线路。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func CreateMultiPathGatewayLineWithContext(ctx context.Context, c *Client, request *CreateMultiPathGatewayLineRequest) (response *CreateMultiPathGatewayLineResponse, err error) {
    if request == nil {
        request = NewCreateMultiPathGatewayLineRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateMultiPathGatewayLine")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateMultiPathGatewayLine require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateMultiPathGatewayLineResponse()
    err = c.Send(request, response)
    return
}

func NewCreateMultiPathGatewaySecretKeyRequest() (request *CreateMultiPathGatewaySecretKeyRequest) {
    request = &CreateMultiPathGatewaySecretKeyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateMultiPathGatewaySecretKey")
    
    
    return
}

func NewCreateMultiPathGatewaySecretKeyResponse() (response *CreateMultiPathGatewaySecretKeyResponse) {
    response = &CreateMultiPathGatewaySecretKeyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateMultiPathGatewaySecretKey
// 通过本接口创建接入多通道安全加速网关的密钥，客户基于接入密钥签名接入多通道安全加速网关。每个站点下只有一个密钥，可用于接入该站点下的所有网关，可通过接口 DescribeMultiPathGatewaySecretKey 查询。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func CreateMultiPathGatewaySecretKey(c *Client, request *CreateMultiPathGatewaySecretKeyRequest) (response *CreateMultiPathGatewaySecretKeyResponse, err error) {
    return CreateMultiPathGatewaySecretKeyWithContext(context.Background(), c, request)
}

// CreateMultiPathGatewaySecretKey
// 通过本接口创建接入多通道安全加速网关的密钥，客户基于接入密钥签名接入多通道安全加速网关。每个站点下只有一个密钥，可用于接入该站点下的所有网关，可通过接口 DescribeMultiPathGatewaySecretKey 查询。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func CreateMultiPathGatewaySecretKeyWithContext(ctx context.Context, c *Client, request *CreateMultiPathGatewaySecretKeyRequest) (response *CreateMultiPathGatewaySecretKeyResponse, err error) {
    if request == nil {
        request = NewCreateMultiPathGatewaySecretKeyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateMultiPathGatewaySecretKey")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateMultiPathGatewaySecretKey require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateMultiPathGatewaySecretKeyResponse()
    err = c.Send(request, response)
    return
}

func NewCreateOriginGroupRequest() (request *CreateOriginGroupRequest) {
    request = &CreateOriginGroupRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateOriginGroup")
    
    
    return
}

func NewCreateOriginGroupResponse() (response *CreateOriginGroupResponse) {
    response = &CreateOriginGroupResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateOriginGroup
// 创建源站组，以源站组的方式管理业务源站。此处配置的源站组可于**添加加速域名**和**四层代理**等功能中引用。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INVALIDPARAMETER_HOSTHEADERINVALID = "InvalidParameter.HostHeaderInvalid"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDORIGINGROUPTYPE = "InvalidParameter.InvalidOriginGroupType"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_ORIGINNAMEEXISTS = "InvalidParameter.OriginNameExists"
//  INVALIDPARAMETER_ORIGINRECORDFORMATERROR = "InvalidParameter.OriginRecordFormatError"
//  INVALIDPARAMETER_ORIGINRECORDWEIGHTVALUE = "InvalidParameter.OriginRecordWeightValue"
//  INVALIDPARAMETER_ORIGINTHIRDPARTYPARAMFORMATERROR = "InvalidParameter.OriginThirdPartyParamFormatError"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_LOADBALANCINGZONEISNOTACTIVE = "OperationDenied.LoadBalancingZoneIsNotActive"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func CreateOriginGroup(c *Client, request *CreateOriginGroupRequest) (response *CreateOriginGroupResponse, err error) {
    return CreateOriginGroupWithContext(context.Background(), c, request)
}

// CreateOriginGroup
// 创建源站组，以源站组的方式管理业务源站。此处配置的源站组可于**添加加速域名**和**四层代理**等功能中引用。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INVALIDPARAMETER_HOSTHEADERINVALID = "InvalidParameter.HostHeaderInvalid"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDORIGINGROUPTYPE = "InvalidParameter.InvalidOriginGroupType"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_ORIGINNAMEEXISTS = "InvalidParameter.OriginNameExists"
//  INVALIDPARAMETER_ORIGINRECORDFORMATERROR = "InvalidParameter.OriginRecordFormatError"
//  INVALIDPARAMETER_ORIGINRECORDWEIGHTVALUE = "InvalidParameter.OriginRecordWeightValue"
//  INVALIDPARAMETER_ORIGINTHIRDPARTYPARAMFORMATERROR = "InvalidParameter.OriginThirdPartyParamFormatError"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_LOADBALANCINGZONEISNOTACTIVE = "OperationDenied.LoadBalancingZoneIsNotActive"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func CreateOriginGroupWithContext(ctx context.Context, c *Client, request *CreateOriginGroupRequest) (response *CreateOriginGroupResponse, err error) {
    if request == nil {
        request = NewCreateOriginGroupRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateOriginGroup")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateOriginGroup require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateOriginGroupResponse()
    err = c.Send(request, response)
    return
}

func NewCreatePlanRequest() (request *CreatePlanRequest) {
    request = &CreatePlanRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreatePlan")
    
    
    return
}

func NewCreatePlanResponse() (response *CreatePlanResponse) {
    response = &CreatePlanResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreatePlan
// 若您需要使用 Edgeone 产品，您需要通过此接口创建计费套餐。
//
// > 创建套餐后，您需要通过 [CreateZone](https://cloud.tencent.com/document/product/1552/80719) 完成创建站点，绑定套餐的流程，Edgeone 才能正常提供服务。
//
// 可能返回的错误码:
//  FAILEDOPERATION_INSUFFICIENTACCOUNTBALANCE = "FailedOperation.InsufficientAccountBalance"
//  INVALIDPARAMETER_INVALIDAUTOUSEVOUCHER = "InvalidParameter.InvalidAutoUseVoucher"
//  INVALIDPARAMETER_INVALIDPERIOD = "InvalidParameter.InvalidPeriod"
//  INVALIDPARAMETER_INVALIDPLANTYPE = "InvalidParameter.InvalidPlanType"
//  INVALIDPARAMETER_INVALIDRENEWFLAG = "InvalidParameter.InvalidRenewFlag"
//  OPERATIONDENIED_PLEASECONTACTBUSINESSPERSONNEL = "OperationDenied.PleaseContactBusinessPersonnel"
func CreatePlan(c *Client, request *CreatePlanRequest) (response *CreatePlanResponse, err error) {
    return CreatePlanWithContext(context.Background(), c, request)
}

// CreatePlan
// 若您需要使用 Edgeone 产品，您需要通过此接口创建计费套餐。
//
// > 创建套餐后，您需要通过 [CreateZone](https://cloud.tencent.com/document/product/1552/80719) 完成创建站点，绑定套餐的流程，Edgeone 才能正常提供服务。
//
// 可能返回的错误码:
//  FAILEDOPERATION_INSUFFICIENTACCOUNTBALANCE = "FailedOperation.InsufficientAccountBalance"
//  INVALIDPARAMETER_INVALIDAUTOUSEVOUCHER = "InvalidParameter.InvalidAutoUseVoucher"
//  INVALIDPARAMETER_INVALIDPERIOD = "InvalidParameter.InvalidPeriod"
//  INVALIDPARAMETER_INVALIDPLANTYPE = "InvalidParameter.InvalidPlanType"
//  INVALIDPARAMETER_INVALIDRENEWFLAG = "InvalidParameter.InvalidRenewFlag"
//  OPERATIONDENIED_PLEASECONTACTBUSINESSPERSONNEL = "OperationDenied.PleaseContactBusinessPersonnel"
func CreatePlanWithContext(ctx context.Context, c *Client, request *CreatePlanRequest) (response *CreatePlanResponse, err error) {
    if request == nil {
        request = NewCreatePlanRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreatePlan")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreatePlan require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreatePlanResponse()
    err = c.Send(request, response)
    return
}

func NewCreatePlanForZoneRequest() (request *CreatePlanForZoneRequest) {
    request = &CreatePlanForZoneRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreatePlanForZone")
    
    
    return
}

func NewCreatePlanForZoneResponse() (response *CreatePlanForZoneResponse) {
    response = &CreatePlanForZoneResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreatePlanForZone
// 为未购买套餐的站点购买套餐
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_INSUFFICIENTACCOUNTBALANCE = "FailedOperation.InsufficientAccountBalance"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_ZONEHASBEENBOUND = "InvalidParameter.ZoneHasBeenBound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DOMAINNOICP = "OperationDenied.DomainNoICP"
func CreatePlanForZone(c *Client, request *CreatePlanForZoneRequest) (response *CreatePlanForZoneResponse, err error) {
    return CreatePlanForZoneWithContext(context.Background(), c, request)
}

// CreatePlanForZone
// 为未购买套餐的站点购买套餐
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_INSUFFICIENTACCOUNTBALANCE = "FailedOperation.InsufficientAccountBalance"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_ZONEHASBEENBOUND = "InvalidParameter.ZoneHasBeenBound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DOMAINNOICP = "OperationDenied.DomainNoICP"
func CreatePlanForZoneWithContext(ctx context.Context, c *Client, request *CreatePlanForZoneRequest) (response *CreatePlanForZoneResponse, err error) {
    if request == nil {
        request = NewCreatePlanForZoneRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreatePlanForZone")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreatePlanForZone require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreatePlanForZoneResponse()
    err = c.Send(request, response)
    return
}

func NewCreatePrefetchTaskRequest() (request *CreatePrefetchTaskRequest) {
    request = &CreatePrefetchTaskRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreatePrefetchTask")
    
    
    return
}

func NewCreatePrefetchTaskResponse() (response *CreatePrefetchTaskResponse) {
    response = &CreatePrefetchTaskResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreatePrefetchTask
// 创建预热任务
//
// 可能返回的错误码:
//  INTERNALERROR_BACKENDERROR = "InternalError.BackendError"
//  INTERNALERROR_DOMAINCONFIG = "InternalError.DomainConfig"
//  INTERNALERROR_FAILEDTOGENERATEURL = "InternalError.FailedToGenerateUrl"
//  INTERNALERROR_QUOTASYSTEM = "InternalError.QuotaSystem"
//  INVALIDPARAMETER_DOMAINNOTFOUND = "InvalidParameter.DomainNotFound"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  INVALIDPARAMETER_TARGET = "InvalidParameter.Target"
//  INVALIDPARAMETER_TASKNOTGENERATED = "InvalidParameter.TaskNotGenerated"
//  INVALIDPARAMETER_UPLOADURL = "InvalidParameter.UploadUrl"
//  LIMITEXCEEDED_BATCHQUOTA = "LimitExceeded.BatchQuota"
//  LIMITEXCEEDED_DAILYQUOTA = "LimitExceeded.DailyQuota"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func CreatePrefetchTask(c *Client, request *CreatePrefetchTaskRequest) (response *CreatePrefetchTaskResponse, err error) {
    return CreatePrefetchTaskWithContext(context.Background(), c, request)
}

// CreatePrefetchTask
// 创建预热任务
//
// 可能返回的错误码:
//  INTERNALERROR_BACKENDERROR = "InternalError.BackendError"
//  INTERNALERROR_DOMAINCONFIG = "InternalError.DomainConfig"
//  INTERNALERROR_FAILEDTOGENERATEURL = "InternalError.FailedToGenerateUrl"
//  INTERNALERROR_QUOTASYSTEM = "InternalError.QuotaSystem"
//  INVALIDPARAMETER_DOMAINNOTFOUND = "InvalidParameter.DomainNotFound"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  INVALIDPARAMETER_TARGET = "InvalidParameter.Target"
//  INVALIDPARAMETER_TASKNOTGENERATED = "InvalidParameter.TaskNotGenerated"
//  INVALIDPARAMETER_UPLOADURL = "InvalidParameter.UploadUrl"
//  LIMITEXCEEDED_BATCHQUOTA = "LimitExceeded.BatchQuota"
//  LIMITEXCEEDED_DAILYQUOTA = "LimitExceeded.DailyQuota"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func CreatePrefetchTaskWithContext(ctx context.Context, c *Client, request *CreatePrefetchTaskRequest) (response *CreatePrefetchTaskResponse, err error) {
    if request == nil {
        request = NewCreatePrefetchTaskRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreatePrefetchTask")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreatePrefetchTask require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreatePrefetchTaskResponse()
    err = c.Send(request, response)
    return
}

func NewCreatePurgeTaskRequest() (request *CreatePurgeTaskRequest) {
    request = &CreatePurgeTaskRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreatePurgeTask")
    
    
    return
}

func NewCreatePurgeTaskResponse() (response *CreatePurgeTaskResponse) {
    response = &CreatePurgeTaskResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreatePurgeTask
// 当源站资源更新，但节点缓存 TTL 未过期时，用户仍会访问到旧的资源，此时可以通过该接口实现节点资源更新。触发更新的方法有以下两种：<li>直接删除：不做任何校验，直接删除节点缓存，用户请求时触发回源拉取；</li><li>标记过期：将节点资源置为过期，用户请求时触发回源校验，即发送带有 If-None-Match 和 If-Modified-Since 头部的 HTTP 条件请求。若源站响应 200，则节点会回源拉取新的资源并更新缓存；若源站响应 304，则节点不会更新缓存；</li>
//
// 
//
// 清除缓存任务详情请查看[清除缓存](https://cloud.tencent.com/document/product/1552/70759)。
//
// 可能返回的错误码:
//  INTERNALERROR_BACKENDERROR = "InternalError.BackendError"
//  INTERNALERROR_DOMAINCONFIG = "InternalError.DomainConfig"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_QUOTASYSTEM = "InternalError.QuotaSystem"
//  INVALIDPARAMETER_DOMAINNOTFOUND = "InvalidParameter.DomainNotFound"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  INVALIDPARAMETER_TARGET = "InvalidParameter.Target"
//  INVALIDPARAMETER_TASKNOTGENERATED = "InvalidParameter.TaskNotGenerated"
//  INVALIDPARAMETER_UPLOADURL = "InvalidParameter.UploadUrl"
//  LIMITEXCEEDED_BATCHQUOTA = "LimitExceeded.BatchQuota"
//  LIMITEXCEEDED_DAILYQUOTA = "LimitExceeded.DailyQuota"
//  LIMITEXCEEDED_PACKNOTALLOW = "LimitExceeded.PackNotAllow"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func CreatePurgeTask(c *Client, request *CreatePurgeTaskRequest) (response *CreatePurgeTaskResponse, err error) {
    return CreatePurgeTaskWithContext(context.Background(), c, request)
}

// CreatePurgeTask
// 当源站资源更新，但节点缓存 TTL 未过期时，用户仍会访问到旧的资源，此时可以通过该接口实现节点资源更新。触发更新的方法有以下两种：<li>直接删除：不做任何校验，直接删除节点缓存，用户请求时触发回源拉取；</li><li>标记过期：将节点资源置为过期，用户请求时触发回源校验，即发送带有 If-None-Match 和 If-Modified-Since 头部的 HTTP 条件请求。若源站响应 200，则节点会回源拉取新的资源并更新缓存；若源站响应 304，则节点不会更新缓存；</li>
//
// 
//
// 清除缓存任务详情请查看[清除缓存](https://cloud.tencent.com/document/product/1552/70759)。
//
// 可能返回的错误码:
//  INTERNALERROR_BACKENDERROR = "InternalError.BackendError"
//  INTERNALERROR_DOMAINCONFIG = "InternalError.DomainConfig"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_QUOTASYSTEM = "InternalError.QuotaSystem"
//  INVALIDPARAMETER_DOMAINNOTFOUND = "InvalidParameter.DomainNotFound"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  INVALIDPARAMETER_TARGET = "InvalidParameter.Target"
//  INVALIDPARAMETER_TASKNOTGENERATED = "InvalidParameter.TaskNotGenerated"
//  INVALIDPARAMETER_UPLOADURL = "InvalidParameter.UploadUrl"
//  LIMITEXCEEDED_BATCHQUOTA = "LimitExceeded.BatchQuota"
//  LIMITEXCEEDED_DAILYQUOTA = "LimitExceeded.DailyQuota"
//  LIMITEXCEEDED_PACKNOTALLOW = "LimitExceeded.PackNotAllow"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func CreatePurgeTaskWithContext(ctx context.Context, c *Client, request *CreatePurgeTaskRequest) (response *CreatePurgeTaskResponse, err error) {
    if request == nil {
        request = NewCreatePurgeTaskRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreatePurgeTask")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreatePurgeTask require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreatePurgeTaskResponse()
    err = c.Send(request, response)
    return
}

func NewCreateRealtimeLogDeliveryTaskRequest() (request *CreateRealtimeLogDeliveryTaskRequest) {
    request = &CreateRealtimeLogDeliveryTaskRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateRealtimeLogDeliveryTask")
    
    
    return
}

func NewCreateRealtimeLogDeliveryTaskResponse() (response *CreateRealtimeLogDeliveryTaskResponse) {
    response = &CreateRealtimeLogDeliveryTaskResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateRealtimeLogDeliveryTask
// 本接口用于创建实时日志投递任务。本接口有如下限制：
//
// - 当数据投递类型（LogType）为站点加速日志（七层访问日志）、四层代理日志、边缘函数运行日志时，同一个实体（七层域名、四层代理实例、边缘函数实例）在同种数据投递类型（LogType）和数据投递区域（Area）的组合下，只能被添加到如下实时日志投递任务类型（TaskType）组合中：
//
//     - 一个推送至腾讯云  CLS 的任务，加上另一个推送至自定义 HTTP(S) 地址的任务；
//
//     - 一个推送至腾讯云  CLS 的任务，加上另一个推送至 AWS S3 兼容对象存储的任务；
//
// - 当数据投递类型（LogType）为速率限制和 CC 攻击防护日志、托管规则日志、自定义规则日志、Bot 管理日志时，同一个实体在同种数据投递类型（LogType）和数据投递区域（Area）的组合下，只能被添加到一个实时日志投递任务中。
//
// 
//
// 建议先通过 [DescribeRealtimeLogDeliveryTasks](https://cloud.tencent.com/document/product/1552/104110)  接口根据实体查询实时日志投递任务列表，检查实体是否已经被添加到另一实时日志投递任务中。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CREATECLSLOGSETFAILED = "FailedOperation.CreateClsLogSetFailed"
//  FAILEDOPERATION_CREATECLSLOGTOPICTASKFAILED = "FailedOperation.CreateClsLogTopicTaskFailed"
//  FAILEDOPERATION_CREATELOGTOPICTASKAUTHFAILURE = "FailedOperation.CreateLogTopicTaskAuthFailure"
//  FAILEDOPERATION_REALTIMELOGAUTHFAILURE = "FailedOperation.RealtimeLogAuthFailure"
//  FAILEDOPERATION_REALTIMELOGNOTFOUND = "FailedOperation.RealtimeLogNotFound"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_INVALIDLOGFORMATFIELDDELIMITER = "InvalidParameter.InvalidLogFormatFieldDelimiter"
//  INVALIDPARAMETER_INVALIDLOGFORMATFORMATTYPE = "InvalidParameter.InvalidLogFormatFormatType"
//  INVALIDPARAMETER_INVALIDLOGFORMATRECORDDELIMITER = "InvalidParameter.InvalidLogFormatRecordDelimiter"
//  INVALIDPARAMETER_REALTIMELOGENTITYALREADYCREATED = "InvalidParameter.RealtimeLogEntityAlreadyCreated"
//  INVALIDPARAMETER_REALTIMELOGINVALIDDELIVERYAREA = "InvalidParameter.RealtimeLogInvalidDeliveryArea"
//  INVALIDPARAMETER_REALTIMELOGINVALIDLOGTYPE = "InvalidParameter.RealtimeLogInvalidLogType"
//  INVALIDPARAMETER_REALTIMELOGINVALIDTASKTYPE = "InvalidParameter.RealtimeLogInvalidTaskType"
//  INVALIDPARAMETER_REALTIMELOGNUMSEXCEEDLIMIT = "InvalidParameter.RealtimeLogNumsExceedLimit"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_CUSTOMLOGFIELDREGEXLIMITEXCEEDED = "LimitExceeded.CustomLogFieldRegexLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func CreateRealtimeLogDeliveryTask(c *Client, request *CreateRealtimeLogDeliveryTaskRequest) (response *CreateRealtimeLogDeliveryTaskResponse, err error) {
    return CreateRealtimeLogDeliveryTaskWithContext(context.Background(), c, request)
}

// CreateRealtimeLogDeliveryTask
// 本接口用于创建实时日志投递任务。本接口有如下限制：
//
// - 当数据投递类型（LogType）为站点加速日志（七层访问日志）、四层代理日志、边缘函数运行日志时，同一个实体（七层域名、四层代理实例、边缘函数实例）在同种数据投递类型（LogType）和数据投递区域（Area）的组合下，只能被添加到如下实时日志投递任务类型（TaskType）组合中：
//
//     - 一个推送至腾讯云  CLS 的任务，加上另一个推送至自定义 HTTP(S) 地址的任务；
//
//     - 一个推送至腾讯云  CLS 的任务，加上另一个推送至 AWS S3 兼容对象存储的任务；
//
// - 当数据投递类型（LogType）为速率限制和 CC 攻击防护日志、托管规则日志、自定义规则日志、Bot 管理日志时，同一个实体在同种数据投递类型（LogType）和数据投递区域（Area）的组合下，只能被添加到一个实时日志投递任务中。
//
// 
//
// 建议先通过 [DescribeRealtimeLogDeliveryTasks](https://cloud.tencent.com/document/product/1552/104110)  接口根据实体查询实时日志投递任务列表，检查实体是否已经被添加到另一实时日志投递任务中。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CREATECLSLOGSETFAILED = "FailedOperation.CreateClsLogSetFailed"
//  FAILEDOPERATION_CREATECLSLOGTOPICTASKFAILED = "FailedOperation.CreateClsLogTopicTaskFailed"
//  FAILEDOPERATION_CREATELOGTOPICTASKAUTHFAILURE = "FailedOperation.CreateLogTopicTaskAuthFailure"
//  FAILEDOPERATION_REALTIMELOGAUTHFAILURE = "FailedOperation.RealtimeLogAuthFailure"
//  FAILEDOPERATION_REALTIMELOGNOTFOUND = "FailedOperation.RealtimeLogNotFound"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_INVALIDLOGFORMATFIELDDELIMITER = "InvalidParameter.InvalidLogFormatFieldDelimiter"
//  INVALIDPARAMETER_INVALIDLOGFORMATFORMATTYPE = "InvalidParameter.InvalidLogFormatFormatType"
//  INVALIDPARAMETER_INVALIDLOGFORMATRECORDDELIMITER = "InvalidParameter.InvalidLogFormatRecordDelimiter"
//  INVALIDPARAMETER_REALTIMELOGENTITYALREADYCREATED = "InvalidParameter.RealtimeLogEntityAlreadyCreated"
//  INVALIDPARAMETER_REALTIMELOGINVALIDDELIVERYAREA = "InvalidParameter.RealtimeLogInvalidDeliveryArea"
//  INVALIDPARAMETER_REALTIMELOGINVALIDLOGTYPE = "InvalidParameter.RealtimeLogInvalidLogType"
//  INVALIDPARAMETER_REALTIMELOGINVALIDTASKTYPE = "InvalidParameter.RealtimeLogInvalidTaskType"
//  INVALIDPARAMETER_REALTIMELOGNUMSEXCEEDLIMIT = "InvalidParameter.RealtimeLogNumsExceedLimit"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_CUSTOMLOGFIELDREGEXLIMITEXCEEDED = "LimitExceeded.CustomLogFieldRegexLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func CreateRealtimeLogDeliveryTaskWithContext(ctx context.Context, c *Client, request *CreateRealtimeLogDeliveryTaskRequest) (response *CreateRealtimeLogDeliveryTaskResponse, err error) {
    if request == nil {
        request = NewCreateRealtimeLogDeliveryTaskRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateRealtimeLogDeliveryTask")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateRealtimeLogDeliveryTask require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateRealtimeLogDeliveryTaskResponse()
    err = c.Send(request, response)
    return
}

func NewCreateRuleRequest() (request *CreateRuleRequest) {
    request = &CreateRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateRule")
    
    
    return
}

func NewCreateRuleResponse() (response *CreateRuleResponse) {
    response = &CreateRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateRule
// 本接口为旧版本创建规则引擎接口，EdgeOne 于 2025 年 1 月 21 日已对规则引擎相关接口全面升级，新版本创建七层加速规则接口详情请参考 [CreateL7AccRules](https://cloud.tencent.com/document/product/1552/115822)。
//
// <p style="color: red;">注意：自 2025 年 1 月 21 日起，旧版接口停止更新迭代，后续新增功能将仅在新版接口中提供，旧版接口支持的原有能力将不受影响。为避免在使用旧版接口时出现数据字段冲突，建议您尽早迁移到新版规则引擎接口。</p>
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACCESSREDIRECTREGEXERROR = "InvalidParameter.AccessRedirectRegexError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONORIGINPRIVATEADDRESS = "InvalidParameter.ErrInvalidActionOriginPrivateAddress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMBADVALUETYPE = "InvalidParameter.ErrInvalidActionParamBadValueType"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITION = "InvalidParameter.ErrInvalidCondition"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONCANNOTONLYCONTAINHOSTWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionCannotOnlyContainHostWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYREGULAR = "InvalidParameter.ErrInvalidConditionValueTooManyRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEZEROLENGTH = "InvalidParameter.ErrInvalidConditionValueZeroLength"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATION = "InvalidParameter.InvalidAuthentication"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPEEXPIRETIME = "InvalidParameter.InvalidAuthenticationTypeExpireTime"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHECONFIGCACHE = "InvalidParameter.InvalidCacheConfigCache"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECT = "InvalidParameter.InvalidUrlRedirect"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_TASKSYSTEMERROR = "InvalidParameter.TaskSystemError"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateRule(c *Client, request *CreateRuleRequest) (response *CreateRuleResponse, err error) {
    return CreateRuleWithContext(context.Background(), c, request)
}

// CreateRule
// 本接口为旧版本创建规则引擎接口，EdgeOne 于 2025 年 1 月 21 日已对规则引擎相关接口全面升级，新版本创建七层加速规则接口详情请参考 [CreateL7AccRules](https://cloud.tencent.com/document/product/1552/115822)。
//
// <p style="color: red;">注意：自 2025 年 1 月 21 日起，旧版接口停止更新迭代，后续新增功能将仅在新版接口中提供，旧版接口支持的原有能力将不受影响。为避免在使用旧版接口时出现数据字段冲突，建议您尽早迁移到新版规则引擎接口。</p>
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACCESSREDIRECTREGEXERROR = "InvalidParameter.AccessRedirectRegexError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONORIGINPRIVATEADDRESS = "InvalidParameter.ErrInvalidActionOriginPrivateAddress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMBADVALUETYPE = "InvalidParameter.ErrInvalidActionParamBadValueType"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITION = "InvalidParameter.ErrInvalidCondition"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONCANNOTONLYCONTAINHOSTWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionCannotOnlyContainHostWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYREGULAR = "InvalidParameter.ErrInvalidConditionValueTooManyRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEZEROLENGTH = "InvalidParameter.ErrInvalidConditionValueZeroLength"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATION = "InvalidParameter.InvalidAuthentication"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPEEXPIRETIME = "InvalidParameter.InvalidAuthenticationTypeExpireTime"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHECONFIGCACHE = "InvalidParameter.InvalidCacheConfigCache"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECT = "InvalidParameter.InvalidUrlRedirect"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_TASKSYSTEMERROR = "InvalidParameter.TaskSystemError"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateRuleWithContext(ctx context.Context, c *Client, request *CreateRuleRequest) (response *CreateRuleResponse, err error) {
    if request == nil {
        request = NewCreateRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateRuleResponse()
    err = c.Send(request, response)
    return
}

func NewCreateSecurityAPIResourceRequest() (request *CreateSecurityAPIResourceRequest) {
    request = &CreateSecurityAPIResourceRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateSecurityAPIResource")
    
    
    return
}

func NewCreateSecurityAPIResourceResponse() (response *CreateSecurityAPIResourceResponse) {
    response = &CreateSecurityAPIResourceResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateSecurityAPIResource
// 用于创建 API 资源。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACCESSREDIRECTREGEXERROR = "InvalidParameter.AccessRedirectRegexError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONORIGINPRIVATEADDRESS = "InvalidParameter.ErrInvalidActionOriginPrivateAddress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMBADVALUETYPE = "InvalidParameter.ErrInvalidActionParamBadValueType"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITION = "InvalidParameter.ErrInvalidCondition"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONCANNOTONLYCONTAINHOSTWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionCannotOnlyContainHostWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYREGULAR = "InvalidParameter.ErrInvalidConditionValueTooManyRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEZEROLENGTH = "InvalidParameter.ErrInvalidConditionValueZeroLength"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATION = "InvalidParameter.InvalidAuthentication"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPEEXPIRETIME = "InvalidParameter.InvalidAuthenticationTypeExpireTime"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHECONFIGCACHE = "InvalidParameter.InvalidCacheConfigCache"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECT = "InvalidParameter.InvalidUrlRedirect"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_TASKSYSTEMERROR = "InvalidParameter.TaskSystemError"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateSecurityAPIResource(c *Client, request *CreateSecurityAPIResourceRequest) (response *CreateSecurityAPIResourceResponse, err error) {
    return CreateSecurityAPIResourceWithContext(context.Background(), c, request)
}

// CreateSecurityAPIResource
// 用于创建 API 资源。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACCESSREDIRECTREGEXERROR = "InvalidParameter.AccessRedirectRegexError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONORIGINPRIVATEADDRESS = "InvalidParameter.ErrInvalidActionOriginPrivateAddress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMBADVALUETYPE = "InvalidParameter.ErrInvalidActionParamBadValueType"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITION = "InvalidParameter.ErrInvalidCondition"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONCANNOTONLYCONTAINHOSTWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionCannotOnlyContainHostWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYREGULAR = "InvalidParameter.ErrInvalidConditionValueTooManyRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEZEROLENGTH = "InvalidParameter.ErrInvalidConditionValueZeroLength"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATION = "InvalidParameter.InvalidAuthentication"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPEEXPIRETIME = "InvalidParameter.InvalidAuthenticationTypeExpireTime"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHECONFIGCACHE = "InvalidParameter.InvalidCacheConfigCache"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECT = "InvalidParameter.InvalidUrlRedirect"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_TASKSYSTEMERROR = "InvalidParameter.TaskSystemError"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateSecurityAPIResourceWithContext(ctx context.Context, c *Client, request *CreateSecurityAPIResourceRequest) (response *CreateSecurityAPIResourceResponse, err error) {
    if request == nil {
        request = NewCreateSecurityAPIResourceRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateSecurityAPIResource")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateSecurityAPIResource require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateSecurityAPIResourceResponse()
    err = c.Send(request, response)
    return
}

func NewCreateSecurityAPIServiceRequest() (request *CreateSecurityAPIServiceRequest) {
    request = &CreateSecurityAPIServiceRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateSecurityAPIService")
    
    
    return
}

func NewCreateSecurityAPIServiceResponse() (response *CreateSecurityAPIServiceResponse) {
    response = &CreateSecurityAPIServiceResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateSecurityAPIService
// 用于创建 API 服务。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACCESSREDIRECTREGEXERROR = "InvalidParameter.AccessRedirectRegexError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONORIGINPRIVATEADDRESS = "InvalidParameter.ErrInvalidActionOriginPrivateAddress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMBADVALUETYPE = "InvalidParameter.ErrInvalidActionParamBadValueType"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITION = "InvalidParameter.ErrInvalidCondition"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONCANNOTONLYCONTAINHOSTWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionCannotOnlyContainHostWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYREGULAR = "InvalidParameter.ErrInvalidConditionValueTooManyRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEZEROLENGTH = "InvalidParameter.ErrInvalidConditionValueZeroLength"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATION = "InvalidParameter.InvalidAuthentication"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPEEXPIRETIME = "InvalidParameter.InvalidAuthenticationTypeExpireTime"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHECONFIGCACHE = "InvalidParameter.InvalidCacheConfigCache"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECT = "InvalidParameter.InvalidUrlRedirect"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_TASKSYSTEMERROR = "InvalidParameter.TaskSystemError"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateSecurityAPIService(c *Client, request *CreateSecurityAPIServiceRequest) (response *CreateSecurityAPIServiceResponse, err error) {
    return CreateSecurityAPIServiceWithContext(context.Background(), c, request)
}

// CreateSecurityAPIService
// 用于创建 API 服务。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACCESSREDIRECTREGEXERROR = "InvalidParameter.AccessRedirectRegexError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONORIGINPRIVATEADDRESS = "InvalidParameter.ErrInvalidActionOriginPrivateAddress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMBADVALUETYPE = "InvalidParameter.ErrInvalidActionParamBadValueType"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITION = "InvalidParameter.ErrInvalidCondition"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONCANNOTONLYCONTAINHOSTWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionCannotOnlyContainHostWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYREGULAR = "InvalidParameter.ErrInvalidConditionValueTooManyRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEZEROLENGTH = "InvalidParameter.ErrInvalidConditionValueZeroLength"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATION = "InvalidParameter.InvalidAuthentication"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPEEXPIRETIME = "InvalidParameter.InvalidAuthenticationTypeExpireTime"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHECONFIGCACHE = "InvalidParameter.InvalidCacheConfigCache"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECT = "InvalidParameter.InvalidUrlRedirect"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_TASKSYSTEMERROR = "InvalidParameter.TaskSystemError"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateSecurityAPIServiceWithContext(ctx context.Context, c *Client, request *CreateSecurityAPIServiceRequest) (response *CreateSecurityAPIServiceResponse, err error) {
    if request == nil {
        request = NewCreateSecurityAPIServiceRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateSecurityAPIService")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateSecurityAPIService require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateSecurityAPIServiceResponse()
    err = c.Send(request, response)
    return
}

func NewCreateSecurityClientAttesterRequest() (request *CreateSecurityClientAttesterRequest) {
    request = &CreateSecurityClientAttesterRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateSecurityClientAttester")
    
    
    return
}

func NewCreateSecurityClientAttesterResponse() (response *CreateSecurityClientAttesterResponse) {
    response = &CreateSecurityClientAttesterResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateSecurityClientAttester
// 创建客户端认证选项。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACCESSREDIRECTREGEXERROR = "InvalidParameter.AccessRedirectRegexError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONORIGINPRIVATEADDRESS = "InvalidParameter.ErrInvalidActionOriginPrivateAddress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMBADVALUETYPE = "InvalidParameter.ErrInvalidActionParamBadValueType"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITION = "InvalidParameter.ErrInvalidCondition"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONCANNOTONLYCONTAINHOSTWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionCannotOnlyContainHostWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYREGULAR = "InvalidParameter.ErrInvalidConditionValueTooManyRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEZEROLENGTH = "InvalidParameter.ErrInvalidConditionValueZeroLength"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATION = "InvalidParameter.InvalidAuthentication"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPEEXPIRETIME = "InvalidParameter.InvalidAuthenticationTypeExpireTime"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHECONFIGCACHE = "InvalidParameter.InvalidCacheConfigCache"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECT = "InvalidParameter.InvalidUrlRedirect"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_TASKSYSTEMERROR = "InvalidParameter.TaskSystemError"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateSecurityClientAttester(c *Client, request *CreateSecurityClientAttesterRequest) (response *CreateSecurityClientAttesterResponse, err error) {
    return CreateSecurityClientAttesterWithContext(context.Background(), c, request)
}

// CreateSecurityClientAttester
// 创建客户端认证选项。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACCESSREDIRECTREGEXERROR = "InvalidParameter.AccessRedirectRegexError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONORIGINPRIVATEADDRESS = "InvalidParameter.ErrInvalidActionOriginPrivateAddress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMBADVALUETYPE = "InvalidParameter.ErrInvalidActionParamBadValueType"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITION = "InvalidParameter.ErrInvalidCondition"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONCANNOTONLYCONTAINHOSTWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionCannotOnlyContainHostWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYREGULAR = "InvalidParameter.ErrInvalidConditionValueTooManyRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEZEROLENGTH = "InvalidParameter.ErrInvalidConditionValueZeroLength"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATION = "InvalidParameter.InvalidAuthentication"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPEEXPIRETIME = "InvalidParameter.InvalidAuthenticationTypeExpireTime"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHECONFIGCACHE = "InvalidParameter.InvalidCacheConfigCache"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECT = "InvalidParameter.InvalidUrlRedirect"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_TASKSYSTEMERROR = "InvalidParameter.TaskSystemError"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateSecurityClientAttesterWithContext(ctx context.Context, c *Client, request *CreateSecurityClientAttesterRequest) (response *CreateSecurityClientAttesterResponse, err error) {
    if request == nil {
        request = NewCreateSecurityClientAttesterRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateSecurityClientAttester")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateSecurityClientAttester require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateSecurityClientAttesterResponse()
    err = c.Send(request, response)
    return
}

func NewCreateSecurityIPGroupRequest() (request *CreateSecurityIPGroupRequest) {
    request = &CreateSecurityIPGroupRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateSecurityIPGroup")
    
    
    return
}

func NewCreateSecurityIPGroupResponse() (response *CreateSecurityIPGroupResponse) {
    response = &CreateSecurityIPGroupResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateSecurityIPGroup
// 创建安全 IP 组
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateSecurityIPGroup(c *Client, request *CreateSecurityIPGroupRequest) (response *CreateSecurityIPGroupResponse, err error) {
    return CreateSecurityIPGroupWithContext(context.Background(), c, request)
}

// CreateSecurityIPGroup
// 创建安全 IP 组
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateSecurityIPGroupWithContext(ctx context.Context, c *Client, request *CreateSecurityIPGroupRequest) (response *CreateSecurityIPGroupResponse, err error) {
    if request == nil {
        request = NewCreateSecurityIPGroupRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateSecurityIPGroup")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateSecurityIPGroup require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateSecurityIPGroupResponse()
    err = c.Send(request, response)
    return
}

func NewCreateSecurityJSInjectionRuleRequest() (request *CreateSecurityJSInjectionRuleRequest) {
    request = &CreateSecurityJSInjectionRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateSecurityJSInjectionRule")
    
    
    return
}

func NewCreateSecurityJSInjectionRuleResponse() (response *CreateSecurityJSInjectionRuleResponse) {
    response = &CreateSecurityJSInjectionRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateSecurityJSInjectionRule
// 创建 JavaScript 注入规则。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateSecurityJSInjectionRule(c *Client, request *CreateSecurityJSInjectionRuleRequest) (response *CreateSecurityJSInjectionRuleResponse, err error) {
    return CreateSecurityJSInjectionRuleWithContext(context.Background(), c, request)
}

// CreateSecurityJSInjectionRule
// 创建 JavaScript 注入规则。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateSecurityJSInjectionRuleWithContext(ctx context.Context, c *Client, request *CreateSecurityJSInjectionRuleRequest) (response *CreateSecurityJSInjectionRuleResponse, err error) {
    if request == nil {
        request = NewCreateSecurityJSInjectionRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateSecurityJSInjectionRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateSecurityJSInjectionRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateSecurityJSInjectionRuleResponse()
    err = c.Send(request, response)
    return
}

func NewCreateSharedCNAMERequest() (request *CreateSharedCNAMERequest) {
    request = &CreateSharedCNAMERequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateSharedCNAME")
    
    
    return
}

func NewCreateSharedCNAMEResponse() (response *CreateSharedCNAMEResponse) {
    response = &CreateSharedCNAMEResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateSharedCNAME
// 用于创建共享 CNAME，该功能白名单内测中。
//
// 可能返回的错误码:
//  INVALIDPARAMETERVALUE_NOTALLOWEDWILDCARDSHAREDCNAME = "InvalidParameterValue.NotAllowedWildcardSharedCNAME"
//  INVALIDPARAMETERVALUE_SHAREDCNAMEPREFIXNOTMATCH = "InvalidParameterValue.SharedCNAMEPrefixNotMatch"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEUNAVAILABLE_SHAREDCNAMEALREADYEXISTS = "ResourceUnavailable.SharedCNAMEAlreadyExists"
func CreateSharedCNAME(c *Client, request *CreateSharedCNAMERequest) (response *CreateSharedCNAMEResponse, err error) {
    return CreateSharedCNAMEWithContext(context.Background(), c, request)
}

// CreateSharedCNAME
// 用于创建共享 CNAME，该功能白名单内测中。
//
// 可能返回的错误码:
//  INVALIDPARAMETERVALUE_NOTALLOWEDWILDCARDSHAREDCNAME = "InvalidParameterValue.NotAllowedWildcardSharedCNAME"
//  INVALIDPARAMETERVALUE_SHAREDCNAMEPREFIXNOTMATCH = "InvalidParameterValue.SharedCNAMEPrefixNotMatch"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEUNAVAILABLE_SHAREDCNAMEALREADYEXISTS = "ResourceUnavailable.SharedCNAMEAlreadyExists"
func CreateSharedCNAMEWithContext(ctx context.Context, c *Client, request *CreateSharedCNAMERequest) (response *CreateSharedCNAMEResponse, err error) {
    if request == nil {
        request = NewCreateSharedCNAMERequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateSharedCNAME")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateSharedCNAME require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateSharedCNAMEResponse()
    err = c.Send(request, response)
    return
}

func NewCreateWebSecurityTemplateRequest() (request *CreateWebSecurityTemplateRequest) {
    request = &CreateWebSecurityTemplateRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateWebSecurityTemplate")
    
    
    return
}

func NewCreateWebSecurityTemplateResponse() (response *CreateWebSecurityTemplateResponse) {
    response = &CreateWebSecurityTemplateResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateWebSecurityTemplate
// 创建安全策略配置模板
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  LIMITEXCEEDED_SECURITY = "LimitExceeded.Security"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateWebSecurityTemplate(c *Client, request *CreateWebSecurityTemplateRequest) (response *CreateWebSecurityTemplateResponse, err error) {
    return CreateWebSecurityTemplateWithContext(context.Background(), c, request)
}

// CreateWebSecurityTemplate
// 创建安全策略配置模板
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  LIMITEXCEEDED_SECURITY = "LimitExceeded.Security"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func CreateWebSecurityTemplateWithContext(ctx context.Context, c *Client, request *CreateWebSecurityTemplateRequest) (response *CreateWebSecurityTemplateResponse, err error) {
    if request == nil {
        request = NewCreateWebSecurityTemplateRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateWebSecurityTemplate")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateWebSecurityTemplate require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateWebSecurityTemplateResponse()
    err = c.Send(request, response)
    return
}

func NewCreateZoneRequest() (request *CreateZoneRequest) {
    request = &CreateZoneRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "CreateZone")
    
    
    return
}

func NewCreateZoneResponse() (response *CreateZoneResponse) {
    response = &CreateZoneResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// CreateZone
// EdgeOne 为您提供 CNAME、NS 和无域名接入三种接入方式，您需要先通过此接口完成站点创建。CNAME 和 NS 接入站点的场景可参考 [从零开始快速接入 EdgeOne](https://cloud.tencent.com/document/product/1552/87601); 无域名接入的场景可参考 [快速启用四层代理服务](https://cloud.tencent.com/document/product/1552/96051)。
//
// 
//
// > 建议您在账号下已存在套餐时调用本接口创建站点，请在入参时传入 PlanId ，直接将站点绑定至该套餐；不传入 PlanId 时，创建出来的站点会处于未激活状态，无法正常服务，您需要通过 [BindZoneToPlan](https://cloud.tencent.com/document/product/1552/83042) 完成套餐绑定之后，站点才可正常提供服务 。若您当前没有可绑定的套餐时，请前往控制台购买套餐完成站点创建。
//
// 可能返回的错误码:
//  DRYRUNOPERATION = "DryRunOperation"
//  FAILEDOPERATION_FAILEDTOCALLDNSPOD = "FailedOperation.FailedToCallDNSPod"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  INVALIDPARAMETER_ZONEHASBEENBOUND = "InvalidParameter.ZoneHasBeenBound"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_ACCESSBLACKLIST = "InvalidParameterValue.AccessBlacklist"
//  INVALIDPARAMETERVALUE_INVALIDTAGVALUE = "InvalidParameterValue.InvalidTagValue"
//  INVALIDPARAMETERVALUE_TOPLEVELDOMAINNOTSUPPORT = "InvalidParameterValue.TopLevelDomainNotSupport"
//  INVALIDPARAMETERVALUE_ZONENAMEINVALID = "InvalidParameterValue.ZoneNameInvalid"
//  INVALIDPARAMETERVALUE_ZONENAMENOTSUPPORTPUNYCODE = "InvalidParameterValue.ZoneNameNotSupportPunyCode"
//  INVALIDPARAMETERVALUE_ZONENAMENOTSUPPORTSUBDOMAIN = "InvalidParameterValue.ZoneNameNotSupportSubDomain"
//  INVALIDPARAMETERVALUE_ZONESAMEASNAME = "InvalidParameterValue.ZoneSameAsName"
//  LIMITEXCEEDED_ZONEBINDPLAN = "LimitExceeded.ZoneBindPlan"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_COMPLIANCEFORBIDDEN = "OperationDenied.ComplianceForbidden"
//  OPERATIONDENIED_DNSPODUNAUTHORIZEDROLEOPERATION = "OperationDenied.DNSPodUnauthorizedRoleOperation"
//  OPERATIONDENIED_DOMAINISBLOCKED = "OperationDenied.DomainIsBlocked"
//  OPERATIONDENIED_RECORDISFORBIDDEN = "OperationDenied.RecordIsForbidden"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEINUSE_ALIASDOMAIN = "ResourceInUse.AliasDomain"
//  RESOURCEINUSE_CNAME = "ResourceInUse.Cname"
//  RESOURCEINUSE_DNS = "ResourceInUse.Dns"
//  RESOURCEINUSE_GENERICHOST = "ResourceInUse.GenericHost"
//  RESOURCEINUSE_HOST = "ResourceInUse.Host"
//  RESOURCEINUSE_NS = "ResourceInUse.NS"
//  RESOURCEINUSE_OTHERS = "ResourceInUse.Others"
//  RESOURCEINUSE_OTHERSALIASDOMAIN = "ResourceInUse.OthersAliasDomain"
//  RESOURCEINUSE_OTHERSCNAME = "ResourceInUse.OthersCname"
//  RESOURCEINUSE_OTHERSHOST = "ResourceInUse.OthersHost"
//  RESOURCEINUSE_OTHERSNS = "ResourceInUse.OthersNS"
//  RESOURCEINUSE_SELFANDOTHERSCNAME = "ResourceInUse.SelfAndOthersCname"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCENOTFOUND_DNSPODDOMAINNOTINACCOUNT = "ResourceNotFound.DNSPodDomainNotInAccount"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func CreateZone(c *Client, request *CreateZoneRequest) (response *CreateZoneResponse, err error) {
    return CreateZoneWithContext(context.Background(), c, request)
}

// CreateZone
// EdgeOne 为您提供 CNAME、NS 和无域名接入三种接入方式，您需要先通过此接口完成站点创建。CNAME 和 NS 接入站点的场景可参考 [从零开始快速接入 EdgeOne](https://cloud.tencent.com/document/product/1552/87601); 无域名接入的场景可参考 [快速启用四层代理服务](https://cloud.tencent.com/document/product/1552/96051)。
//
// 
//
// > 建议您在账号下已存在套餐时调用本接口创建站点，请在入参时传入 PlanId ，直接将站点绑定至该套餐；不传入 PlanId 时，创建出来的站点会处于未激活状态，无法正常服务，您需要通过 [BindZoneToPlan](https://cloud.tencent.com/document/product/1552/83042) 完成套餐绑定之后，站点才可正常提供服务 。若您当前没有可绑定的套餐时，请前往控制台购买套餐完成站点创建。
//
// 可能返回的错误码:
//  DRYRUNOPERATION = "DryRunOperation"
//  FAILEDOPERATION_FAILEDTOCALLDNSPOD = "FailedOperation.FailedToCallDNSPod"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  INVALIDPARAMETER_ZONEHASBEENBOUND = "InvalidParameter.ZoneHasBeenBound"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_ACCESSBLACKLIST = "InvalidParameterValue.AccessBlacklist"
//  INVALIDPARAMETERVALUE_INVALIDTAGVALUE = "InvalidParameterValue.InvalidTagValue"
//  INVALIDPARAMETERVALUE_TOPLEVELDOMAINNOTSUPPORT = "InvalidParameterValue.TopLevelDomainNotSupport"
//  INVALIDPARAMETERVALUE_ZONENAMEINVALID = "InvalidParameterValue.ZoneNameInvalid"
//  INVALIDPARAMETERVALUE_ZONENAMENOTSUPPORTPUNYCODE = "InvalidParameterValue.ZoneNameNotSupportPunyCode"
//  INVALIDPARAMETERVALUE_ZONENAMENOTSUPPORTSUBDOMAIN = "InvalidParameterValue.ZoneNameNotSupportSubDomain"
//  INVALIDPARAMETERVALUE_ZONESAMEASNAME = "InvalidParameterValue.ZoneSameAsName"
//  LIMITEXCEEDED_ZONEBINDPLAN = "LimitExceeded.ZoneBindPlan"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_COMPLIANCEFORBIDDEN = "OperationDenied.ComplianceForbidden"
//  OPERATIONDENIED_DNSPODUNAUTHORIZEDROLEOPERATION = "OperationDenied.DNSPodUnauthorizedRoleOperation"
//  OPERATIONDENIED_DOMAINISBLOCKED = "OperationDenied.DomainIsBlocked"
//  OPERATIONDENIED_RECORDISFORBIDDEN = "OperationDenied.RecordIsForbidden"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEINUSE_ALIASDOMAIN = "ResourceInUse.AliasDomain"
//  RESOURCEINUSE_CNAME = "ResourceInUse.Cname"
//  RESOURCEINUSE_DNS = "ResourceInUse.Dns"
//  RESOURCEINUSE_GENERICHOST = "ResourceInUse.GenericHost"
//  RESOURCEINUSE_HOST = "ResourceInUse.Host"
//  RESOURCEINUSE_NS = "ResourceInUse.NS"
//  RESOURCEINUSE_OTHERS = "ResourceInUse.Others"
//  RESOURCEINUSE_OTHERSALIASDOMAIN = "ResourceInUse.OthersAliasDomain"
//  RESOURCEINUSE_OTHERSCNAME = "ResourceInUse.OthersCname"
//  RESOURCEINUSE_OTHERSHOST = "ResourceInUse.OthersHost"
//  RESOURCEINUSE_OTHERSNS = "ResourceInUse.OthersNS"
//  RESOURCEINUSE_SELFANDOTHERSCNAME = "ResourceInUse.SelfAndOthersCname"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCENOTFOUND_DNSPODDOMAINNOTINACCOUNT = "ResourceNotFound.DNSPodDomainNotInAccount"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func CreateZoneWithContext(ctx context.Context, c *Client, request *CreateZoneRequest) (response *CreateZoneResponse, err error) {
    if request == nil {
        request = NewCreateZoneRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "CreateZone")
    
    if c.GetCredential() == nil {
        return nil, errors.New("CreateZone require credential")
    }

    request.SetContext(ctx)
    
    response = NewCreateZoneResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteAccelerationDomainsRequest() (request *DeleteAccelerationDomainsRequest) {
    request = &DeleteAccelerationDomainsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteAccelerationDomains")
    
    
    return
}

func NewDeleteAccelerationDomainsResponse() (response *DeleteAccelerationDomainsResponse) {
    response = &DeleteAccelerationDomainsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteAccelerationDomains
// 批量删除加速域名
//
// 可能返回的错误码:
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  INVALIDPARAMETERVALUE_INVALIDDOMAINSTATUS = "InvalidParameterValue.InvalidDomainStatus"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEUNAVAILABLE_DOMAINNOTFOUND = "ResourceUnavailable.DomainNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteAccelerationDomains(c *Client, request *DeleteAccelerationDomainsRequest) (response *DeleteAccelerationDomainsResponse, err error) {
    return DeleteAccelerationDomainsWithContext(context.Background(), c, request)
}

// DeleteAccelerationDomains
// 批量删除加速域名
//
// 可能返回的错误码:
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  INVALIDPARAMETERVALUE_INVALIDDOMAINSTATUS = "InvalidParameterValue.InvalidDomainStatus"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEUNAVAILABLE_DOMAINNOTFOUND = "ResourceUnavailable.DomainNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteAccelerationDomainsWithContext(ctx context.Context, c *Client, request *DeleteAccelerationDomainsRequest) (response *DeleteAccelerationDomainsResponse, err error) {
    if request == nil {
        request = NewDeleteAccelerationDomainsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteAccelerationDomains")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteAccelerationDomains require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteAccelerationDomainsResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteAliasDomainRequest() (request *DeleteAliasDomainRequest) {
    request = &DeleteAliasDomainRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteAliasDomain")
    
    
    return
}

func NewDeleteAliasDomainResponse() (response *DeleteAliasDomainResponse) {
    response = &DeleteAliasDomainResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteAliasDomain
// 删除别称域名。
//
// 该功能仅企业版套餐支持，并且该功能当前仍在内测中，如需使用，请[联系我们](https://cloud.tencent.com/online-service?from=connect-us)。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
func DeleteAliasDomain(c *Client, request *DeleteAliasDomainRequest) (response *DeleteAliasDomainResponse, err error) {
    return DeleteAliasDomainWithContext(context.Background(), c, request)
}

// DeleteAliasDomain
// 删除别称域名。
//
// 该功能仅企业版套餐支持，并且该功能当前仍在内测中，如需使用，请[联系我们](https://cloud.tencent.com/online-service?from=connect-us)。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
func DeleteAliasDomainWithContext(ctx context.Context, c *Client, request *DeleteAliasDomainRequest) (response *DeleteAliasDomainResponse, err error) {
    if request == nil {
        request = NewDeleteAliasDomainRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteAliasDomain")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteAliasDomain require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteAliasDomainResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteApplicationProxyRequest() (request *DeleteApplicationProxyRequest) {
    request = &DeleteApplicationProxyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteApplicationProxy")
    
    
    return
}

func NewDeleteApplicationProxyResponse() (response *DeleteApplicationProxyResponse) {
    response = &DeleteApplicationProxyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteApplicationProxy
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [删除四层代理实例
//
// ](https://cloud.tencent.com/document/product/1552/103415) 。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteApplicationProxy(c *Client, request *DeleteApplicationProxyRequest) (response *DeleteApplicationProxyResponse, err error) {
    return DeleteApplicationProxyWithContext(context.Background(), c, request)
}

// DeleteApplicationProxy
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [删除四层代理实例
//
// ](https://cloud.tencent.com/document/product/1552/103415) 。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteApplicationProxyWithContext(ctx context.Context, c *Client, request *DeleteApplicationProxyRequest) (response *DeleteApplicationProxyResponse, err error) {
    if request == nil {
        request = NewDeleteApplicationProxyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteApplicationProxy")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteApplicationProxy require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteApplicationProxyResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteApplicationProxyRuleRequest() (request *DeleteApplicationProxyRuleRequest) {
    request = &DeleteApplicationProxyRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteApplicationProxyRule")
    
    
    return
}

func NewDeleteApplicationProxyRuleResponse() (response *DeleteApplicationProxyRuleResponse) {
    response = &DeleteApplicationProxyRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteApplicationProxyRule
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [删除四层代理转发规则](https://cloud.tencent.com/document/product/1552/103414) 。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNSUPPORTEDOPERATION = "UnsupportedOperation"
func DeleteApplicationProxyRule(c *Client, request *DeleteApplicationProxyRuleRequest) (response *DeleteApplicationProxyRuleResponse, err error) {
    return DeleteApplicationProxyRuleWithContext(context.Background(), c, request)
}

// DeleteApplicationProxyRule
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [删除四层代理转发规则](https://cloud.tencent.com/document/product/1552/103414) 。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNSUPPORTEDOPERATION = "UnsupportedOperation"
func DeleteApplicationProxyRuleWithContext(ctx context.Context, c *Client, request *DeleteApplicationProxyRuleRequest) (response *DeleteApplicationProxyRuleResponse, err error) {
    if request == nil {
        request = NewDeleteApplicationProxyRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteApplicationProxyRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteApplicationProxyRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteApplicationProxyRuleResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteContentIdentifierRequest() (request *DeleteContentIdentifierRequest) {
    request = &DeleteContentIdentifierRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteContentIdentifier")
    
    
    return
}

func NewDeleteContentIdentifierResponse() (response *DeleteContentIdentifierResponse) {
    response = &DeleteContentIdentifierResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteContentIdentifier
// 删除指定的内容标识符。该功能仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNSUPPORTEDOPERATION = "UnsupportedOperation"
func DeleteContentIdentifier(c *Client, request *DeleteContentIdentifierRequest) (response *DeleteContentIdentifierResponse, err error) {
    return DeleteContentIdentifierWithContext(context.Background(), c, request)
}

// DeleteContentIdentifier
// 删除指定的内容标识符。该功能仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNSUPPORTEDOPERATION = "UnsupportedOperation"
func DeleteContentIdentifierWithContext(ctx context.Context, c *Client, request *DeleteContentIdentifierRequest) (response *DeleteContentIdentifierResponse, err error) {
    if request == nil {
        request = NewDeleteContentIdentifierRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteContentIdentifier")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteContentIdentifier require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteContentIdentifierResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteCustomErrorPageRequest() (request *DeleteCustomErrorPageRequest) {
    request = &DeleteCustomErrorPageRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteCustomErrorPage")
    
    
    return
}

func NewDeleteCustomErrorPageResponse() (response *DeleteCustomErrorPageResponse) {
    response = &DeleteCustomErrorPageResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteCustomErrorPage
// 删除自定义错误页面。
//
// 可能返回的错误码:
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
func DeleteCustomErrorPage(c *Client, request *DeleteCustomErrorPageRequest) (response *DeleteCustomErrorPageResponse, err error) {
    return DeleteCustomErrorPageWithContext(context.Background(), c, request)
}

// DeleteCustomErrorPage
// 删除自定义错误页面。
//
// 可能返回的错误码:
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
func DeleteCustomErrorPageWithContext(ctx context.Context, c *Client, request *DeleteCustomErrorPageRequest) (response *DeleteCustomErrorPageResponse, err error) {
    if request == nil {
        request = NewDeleteCustomErrorPageRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteCustomErrorPage")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteCustomErrorPage require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteCustomErrorPageResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteDnsRecordsRequest() (request *DeleteDnsRecordsRequest) {
    request = &DeleteDnsRecordsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteDnsRecords")
    
    
    return
}

func NewDeleteDnsRecordsResponse() (response *DeleteDnsRecordsResponse) {
    response = &DeleteDnsRecordsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteDnsRecords
// 您可以用本接口批量删除 DNS 记录。
//
// 可能返回的错误码:
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
func DeleteDnsRecords(c *Client, request *DeleteDnsRecordsRequest) (response *DeleteDnsRecordsResponse, err error) {
    return DeleteDnsRecordsWithContext(context.Background(), c, request)
}

// DeleteDnsRecords
// 您可以用本接口批量删除 DNS 记录。
//
// 可能返回的错误码:
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
func DeleteDnsRecordsWithContext(ctx context.Context, c *Client, request *DeleteDnsRecordsRequest) (response *DeleteDnsRecordsResponse, err error) {
    if request == nil {
        request = NewDeleteDnsRecordsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteDnsRecords")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteDnsRecords require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteDnsRecordsResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteFunctionRequest() (request *DeleteFunctionRequest) {
    request = &DeleteFunctionRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteFunction")
    
    
    return
}

func NewDeleteFunctionResponse() (response *DeleteFunctionResponse) {
    response = &DeleteFunctionResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteFunction
// 删除边缘函数，删除后函数无法恢复，关联的触发规则会一并删除。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_RULEOPERATIONCONFLICT = "FailedOperation.RuleOperationConflict"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func DeleteFunction(c *Client, request *DeleteFunctionRequest) (response *DeleteFunctionResponse, err error) {
    return DeleteFunctionWithContext(context.Background(), c, request)
}

// DeleteFunction
// 删除边缘函数，删除后函数无法恢复，关联的触发规则会一并删除。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_RULEOPERATIONCONFLICT = "FailedOperation.RuleOperationConflict"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func DeleteFunctionWithContext(ctx context.Context, c *Client, request *DeleteFunctionRequest) (response *DeleteFunctionResponse, err error) {
    if request == nil {
        request = NewDeleteFunctionRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteFunction")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteFunction require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteFunctionResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteFunctionRulesRequest() (request *DeleteFunctionRulesRequest) {
    request = &DeleteFunctionRulesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteFunctionRules")
    
    
    return
}

func NewDeleteFunctionRulesResponse() (response *DeleteFunctionRulesResponse) {
    response = &DeleteFunctionRulesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteFunctionRules
// 删除边缘函数触发规则。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_RULEOPERATIONCONFLICT = "FailedOperation.RuleOperationConflict"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  RESOURCEUNAVAILABLE_RULENOTFOUND = "ResourceUnavailable.RuleNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func DeleteFunctionRules(c *Client, request *DeleteFunctionRulesRequest) (response *DeleteFunctionRulesResponse, err error) {
    return DeleteFunctionRulesWithContext(context.Background(), c, request)
}

// DeleteFunctionRules
// 删除边缘函数触发规则。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_RULEOPERATIONCONFLICT = "FailedOperation.RuleOperationConflict"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  RESOURCEUNAVAILABLE_RULENOTFOUND = "ResourceUnavailable.RuleNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func DeleteFunctionRulesWithContext(ctx context.Context, c *Client, request *DeleteFunctionRulesRequest) (response *DeleteFunctionRulesResponse, err error) {
    if request == nil {
        request = NewDeleteFunctionRulesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteFunctionRules")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteFunctionRules require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteFunctionRulesResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteJustInTimeTranscodeTemplatesRequest() (request *DeleteJustInTimeTranscodeTemplatesRequest) {
    request = &DeleteJustInTimeTranscodeTemplatesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteJustInTimeTranscodeTemplates")
    
    
    return
}

func NewDeleteJustInTimeTranscodeTemplatesResponse() (response *DeleteJustInTimeTranscodeTemplatesResponse) {
    response = &DeleteJustInTimeTranscodeTemplatesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteJustInTimeTranscodeTemplates
// 根据站点 id 下唯一的模板标识，删除相应的即时转码模板。
//
// 可能返回的错误码:
//  FAILEDOPERATION_PRODUCTDISCONTINUED = "FailedOperation.ProductDiscontinued"
//  FAILEDOPERATION_PRODUCTNOTACTIVATED = "FailedOperation.ProductNotActivated"
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_TEMPLATENOTCUSTOM = "InvalidParameterValue.TemplateNotCustom"
//  INVALIDPARAMETERVALUE_TEMPLATENOTFOUND = "InvalidParameterValue.TemplateNotFound"
//  INVALIDPARAMETERVALUE_ZONENOTFOUND = "InvalidParameterValue.ZoneNotFound"
//  INVALIDPARAMETERVALUE_ZONEPAUSED = "InvalidParameterValue.ZonePaused"
//  MISSINGPARAMETER = "MissingParameter"
//  UNAUTHORIZEDOPERATION = "UnauthorizedOperation"
func DeleteJustInTimeTranscodeTemplates(c *Client, request *DeleteJustInTimeTranscodeTemplatesRequest) (response *DeleteJustInTimeTranscodeTemplatesResponse, err error) {
    return DeleteJustInTimeTranscodeTemplatesWithContext(context.Background(), c, request)
}

// DeleteJustInTimeTranscodeTemplates
// 根据站点 id 下唯一的模板标识，删除相应的即时转码模板。
//
// 可能返回的错误码:
//  FAILEDOPERATION_PRODUCTDISCONTINUED = "FailedOperation.ProductDiscontinued"
//  FAILEDOPERATION_PRODUCTNOTACTIVATED = "FailedOperation.ProductNotActivated"
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_TEMPLATENOTCUSTOM = "InvalidParameterValue.TemplateNotCustom"
//  INVALIDPARAMETERVALUE_TEMPLATENOTFOUND = "InvalidParameterValue.TemplateNotFound"
//  INVALIDPARAMETERVALUE_ZONENOTFOUND = "InvalidParameterValue.ZoneNotFound"
//  INVALIDPARAMETERVALUE_ZONEPAUSED = "InvalidParameterValue.ZonePaused"
//  MISSINGPARAMETER = "MissingParameter"
//  UNAUTHORIZEDOPERATION = "UnauthorizedOperation"
func DeleteJustInTimeTranscodeTemplatesWithContext(ctx context.Context, c *Client, request *DeleteJustInTimeTranscodeTemplatesRequest) (response *DeleteJustInTimeTranscodeTemplatesResponse, err error) {
    if request == nil {
        request = NewDeleteJustInTimeTranscodeTemplatesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteJustInTimeTranscodeTemplates")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteJustInTimeTranscodeTemplates require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteJustInTimeTranscodeTemplatesResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteL4ProxyRequest() (request *DeleteL4ProxyRequest) {
    request = &DeleteL4ProxyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteL4Proxy")
    
    
    return
}

func NewDeleteL4ProxyResponse() (response *DeleteL4ProxyResponse) {
    response = &DeleteL4ProxyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteL4Proxy
// 用于删除四层代理实例。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteL4Proxy(c *Client, request *DeleteL4ProxyRequest) (response *DeleteL4ProxyResponse, err error) {
    return DeleteL4ProxyWithContext(context.Background(), c, request)
}

// DeleteL4Proxy
// 用于删除四层代理实例。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteL4ProxyWithContext(ctx context.Context, c *Client, request *DeleteL4ProxyRequest) (response *DeleteL4ProxyResponse, err error) {
    if request == nil {
        request = NewDeleteL4ProxyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteL4Proxy")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteL4Proxy require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteL4ProxyResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteL4ProxyRulesRequest() (request *DeleteL4ProxyRulesRequest) {
    request = &DeleteL4ProxyRulesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteL4ProxyRules")
    
    
    return
}

func NewDeleteL4ProxyRulesResponse() (response *DeleteL4ProxyRulesResponse) {
    response = &DeleteL4ProxyRulesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteL4ProxyRules
// 用于删除四层代理转发规则，支持单条或者批量操作。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNSUPPORTEDOPERATION = "UnsupportedOperation"
func DeleteL4ProxyRules(c *Client, request *DeleteL4ProxyRulesRequest) (response *DeleteL4ProxyRulesResponse, err error) {
    return DeleteL4ProxyRulesWithContext(context.Background(), c, request)
}

// DeleteL4ProxyRules
// 用于删除四层代理转发规则，支持单条或者批量操作。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNSUPPORTEDOPERATION = "UnsupportedOperation"
func DeleteL4ProxyRulesWithContext(ctx context.Context, c *Client, request *DeleteL4ProxyRulesRequest) (response *DeleteL4ProxyRulesResponse, err error) {
    if request == nil {
        request = NewDeleteL4ProxyRulesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteL4ProxyRules")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteL4ProxyRules require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteL4ProxyRulesResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteL7AccRulesRequest() (request *DeleteL7AccRulesRequest) {
    request = &DeleteL7AccRulesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteL7AccRules")
    
    
    return
}

func NewDeleteL7AccRulesResponse() (response *DeleteL7AccRulesResponse) {
    response = &DeleteL7AccRulesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteL7AccRules
// 本接口用于删除[规则引擎](https://cloud.tencent.com/document/product/1552/70901)的规则，支持批量删除。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteL7AccRules(c *Client, request *DeleteL7AccRulesRequest) (response *DeleteL7AccRulesResponse, err error) {
    return DeleteL7AccRulesWithContext(context.Background(), c, request)
}

// DeleteL7AccRules
// 本接口用于删除[规则引擎](https://cloud.tencent.com/document/product/1552/70901)的规则，支持批量删除。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteL7AccRulesWithContext(ctx context.Context, c *Client, request *DeleteL7AccRulesRequest) (response *DeleteL7AccRulesResponse, err error) {
    if request == nil {
        request = NewDeleteL7AccRulesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteL7AccRules")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteL7AccRules require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteL7AccRulesResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteLoadBalancerRequest() (request *DeleteLoadBalancerRequest) {
    request = &DeleteLoadBalancerRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteLoadBalancer")
    
    
    return
}

func NewDeleteLoadBalancerResponse() (response *DeleteLoadBalancerResponse) {
    response = &DeleteLoadBalancerResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteLoadBalancer
// 删除负载均衡实例，若负载均衡示例被其他服务（例如：四层代理等）引用的时候，示例无法被删除，需要先解除引用关系。负载均衡功能内测中，如您需要使用请 [联系我们](https://cloud.tencent.com/online-service)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_LOADBALANCERUSEDINL4PROXY = "InvalidParameter.LoadBalancerUsedInL4Proxy"
//  INVALIDPARAMETER_LOADBALANCERUSEDINL7DOMAIN = "InvalidParameter.LoadBalancerUsedInL7Domain"
//  INVALIDPARAMETER_LOADBALANCERUSEDINRULEENGINE = "InvalidParameter.LoadBalancerUsedInRuleEngine"
func DeleteLoadBalancer(c *Client, request *DeleteLoadBalancerRequest) (response *DeleteLoadBalancerResponse, err error) {
    return DeleteLoadBalancerWithContext(context.Background(), c, request)
}

// DeleteLoadBalancer
// 删除负载均衡实例，若负载均衡示例被其他服务（例如：四层代理等）引用的时候，示例无法被删除，需要先解除引用关系。负载均衡功能内测中，如您需要使用请 [联系我们](https://cloud.tencent.com/online-service)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_LOADBALANCERUSEDINL4PROXY = "InvalidParameter.LoadBalancerUsedInL4Proxy"
//  INVALIDPARAMETER_LOADBALANCERUSEDINL7DOMAIN = "InvalidParameter.LoadBalancerUsedInL7Domain"
//  INVALIDPARAMETER_LOADBALANCERUSEDINRULEENGINE = "InvalidParameter.LoadBalancerUsedInRuleEngine"
func DeleteLoadBalancerWithContext(ctx context.Context, c *Client, request *DeleteLoadBalancerRequest) (response *DeleteLoadBalancerResponse, err error) {
    if request == nil {
        request = NewDeleteLoadBalancerRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteLoadBalancer")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteLoadBalancer require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteLoadBalancerResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteMultiPathGatewayRequest() (request *DeleteMultiPathGatewayRequest) {
    request = &DeleteMultiPathGatewayRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteMultiPathGateway")
    
    
    return
}

func NewDeleteMultiPathGatewayResponse() (response *DeleteMultiPathGatewayResponse) {
    response = &DeleteMultiPathGatewayResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteMultiPathGateway
// 通过本接口删除多通道安全加速网关，包括自有网关和云上网关。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DeleteMultiPathGateway(c *Client, request *DeleteMultiPathGatewayRequest) (response *DeleteMultiPathGatewayResponse, err error) {
    return DeleteMultiPathGatewayWithContext(context.Background(), c, request)
}

// DeleteMultiPathGateway
// 通过本接口删除多通道安全加速网关，包括自有网关和云上网关。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DeleteMultiPathGatewayWithContext(ctx context.Context, c *Client, request *DeleteMultiPathGatewayRequest) (response *DeleteMultiPathGatewayResponse, err error) {
    if request == nil {
        request = NewDeleteMultiPathGatewayRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteMultiPathGateway")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteMultiPathGateway require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteMultiPathGatewayResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteMultiPathGatewayLineRequest() (request *DeleteMultiPathGatewayLineRequest) {
    request = &DeleteMultiPathGatewayLineRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteMultiPathGatewayLine")
    
    
    return
}

func NewDeleteMultiPathGatewayLineResponse() (response *DeleteMultiPathGatewayLineResponse) {
    response = &DeleteMultiPathGatewayLineResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteMultiPathGatewayLine
// 通过本接口删除接入多通道安全加速网关的线路，仅自定义线路支持删除。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DeleteMultiPathGatewayLine(c *Client, request *DeleteMultiPathGatewayLineRequest) (response *DeleteMultiPathGatewayLineResponse, err error) {
    return DeleteMultiPathGatewayLineWithContext(context.Background(), c, request)
}

// DeleteMultiPathGatewayLine
// 通过本接口删除接入多通道安全加速网关的线路，仅自定义线路支持删除。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DeleteMultiPathGatewayLineWithContext(ctx context.Context, c *Client, request *DeleteMultiPathGatewayLineRequest) (response *DeleteMultiPathGatewayLineResponse, err error) {
    if request == nil {
        request = NewDeleteMultiPathGatewayLineRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteMultiPathGatewayLine")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteMultiPathGatewayLine require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteMultiPathGatewayLineResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteOriginGroupRequest() (request *DeleteOriginGroupRequest) {
    request = &DeleteOriginGroupRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteOriginGroup")
    
    
    return
}

func NewDeleteOriginGroupResponse() (response *DeleteOriginGroupResponse) {
    response = &DeleteOriginGroupResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteOriginGroup
// 删除源站组，若源站组仍然被服务（例如：四层代理，域名服务，负载均衡，规则引起）引用，将不允许删除。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ORIGINGROUPACCELERATIONDOMAINUSED = "OperationDenied.OriginGroupAccelerationDomainUsed"
//  OPERATIONDENIED_ORIGINGROUPL4USED = "OperationDenied.OriginGroupL4Used"
//  OPERATIONDENIED_ORIGINGROUPLBUSED = "OperationDenied.OriginGroupLBUsed"
//  OPERATIONDENIED_ORIGINGROUPRULEENGINEUSED = "OperationDenied.OriginGroupRuleEngineUsed"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteOriginGroup(c *Client, request *DeleteOriginGroupRequest) (response *DeleteOriginGroupResponse, err error) {
    return DeleteOriginGroupWithContext(context.Background(), c, request)
}

// DeleteOriginGroup
// 删除源站组，若源站组仍然被服务（例如：四层代理，域名服务，负载均衡，规则引起）引用，将不允许删除。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ORIGINGROUPACCELERATIONDOMAINUSED = "OperationDenied.OriginGroupAccelerationDomainUsed"
//  OPERATIONDENIED_ORIGINGROUPL4USED = "OperationDenied.OriginGroupL4Used"
//  OPERATIONDENIED_ORIGINGROUPLBUSED = "OperationDenied.OriginGroupLBUsed"
//  OPERATIONDENIED_ORIGINGROUPRULEENGINEUSED = "OperationDenied.OriginGroupRuleEngineUsed"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteOriginGroupWithContext(ctx context.Context, c *Client, request *DeleteOriginGroupRequest) (response *DeleteOriginGroupResponse, err error) {
    if request == nil {
        request = NewDeleteOriginGroupRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteOriginGroup")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteOriginGroup require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteOriginGroupResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteRealtimeLogDeliveryTaskRequest() (request *DeleteRealtimeLogDeliveryTaskRequest) {
    request = &DeleteRealtimeLogDeliveryTaskRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteRealtimeLogDeliveryTask")
    
    
    return
}

func NewDeleteRealtimeLogDeliveryTaskResponse() (response *DeleteRealtimeLogDeliveryTaskResponse) {
    response = &DeleteRealtimeLogDeliveryTaskResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteRealtimeLogDeliveryTask
// 通过本接口删除实时日志投递任务。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
func DeleteRealtimeLogDeliveryTask(c *Client, request *DeleteRealtimeLogDeliveryTaskRequest) (response *DeleteRealtimeLogDeliveryTaskResponse, err error) {
    return DeleteRealtimeLogDeliveryTaskWithContext(context.Background(), c, request)
}

// DeleteRealtimeLogDeliveryTask
// 通过本接口删除实时日志投递任务。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
func DeleteRealtimeLogDeliveryTaskWithContext(ctx context.Context, c *Client, request *DeleteRealtimeLogDeliveryTaskRequest) (response *DeleteRealtimeLogDeliveryTaskResponse, err error) {
    if request == nil {
        request = NewDeleteRealtimeLogDeliveryTaskRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteRealtimeLogDeliveryTask")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteRealtimeLogDeliveryTask require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteRealtimeLogDeliveryTaskResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteRulesRequest() (request *DeleteRulesRequest) {
    request = &DeleteRulesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteRules")
    
    
    return
}

func NewDeleteRulesResponse() (response *DeleteRulesResponse) {
    response = &DeleteRulesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteRules
// 本接口为旧版本删除规则引擎接口，EdgeOne 于 2025 年 1 月 21 日已对规则引擎相关接口全面升级，新版本删除七层加速规则接口详情请参考 [DeleteL7AccRules](https://cloud.tencent.com/document/product/1552/115821)。
//
// <p style="color: red;">注意：自 2025 年 1 月 21 日起，旧版接口停止更新迭代，后续新增功能将仅在新版接口中提供，旧版接口支持的原有能力将不受影响。为避免在使用旧版接口时出现数据字段冲突，建议您尽早迁移到新版规则引擎接口。</p>
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteRules(c *Client, request *DeleteRulesRequest) (response *DeleteRulesResponse, err error) {
    return DeleteRulesWithContext(context.Background(), c, request)
}

// DeleteRules
// 本接口为旧版本删除规则引擎接口，EdgeOne 于 2025 年 1 月 21 日已对规则引擎相关接口全面升级，新版本删除七层加速规则接口详情请参考 [DeleteL7AccRules](https://cloud.tencent.com/document/product/1552/115821)。
//
// <p style="color: red;">注意：自 2025 年 1 月 21 日起，旧版接口停止更新迭代，后续新增功能将仅在新版接口中提供，旧版接口支持的原有能力将不受影响。为避免在使用旧版接口时出现数据字段冲突，建议您尽早迁移到新版规则引擎接口。</p>
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteRulesWithContext(ctx context.Context, c *Client, request *DeleteRulesRequest) (response *DeleteRulesResponse, err error) {
    if request == nil {
        request = NewDeleteRulesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteRules")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteRules require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteRulesResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteSecurityAPIResourceRequest() (request *DeleteSecurityAPIResourceRequest) {
    request = &DeleteSecurityAPIResourceRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteSecurityAPIResource")
    
    
    return
}

func NewDeleteSecurityAPIResourceResponse() (response *DeleteSecurityAPIResourceResponse) {
    response = &DeleteSecurityAPIResourceResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteSecurityAPIResource
// 用于删除 API 资源。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteSecurityAPIResource(c *Client, request *DeleteSecurityAPIResourceRequest) (response *DeleteSecurityAPIResourceResponse, err error) {
    return DeleteSecurityAPIResourceWithContext(context.Background(), c, request)
}

// DeleteSecurityAPIResource
// 用于删除 API 资源。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteSecurityAPIResourceWithContext(ctx context.Context, c *Client, request *DeleteSecurityAPIResourceRequest) (response *DeleteSecurityAPIResourceResponse, err error) {
    if request == nil {
        request = NewDeleteSecurityAPIResourceRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteSecurityAPIResource")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteSecurityAPIResource require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteSecurityAPIResourceResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteSecurityAPIServiceRequest() (request *DeleteSecurityAPIServiceRequest) {
    request = &DeleteSecurityAPIServiceRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteSecurityAPIService")
    
    
    return
}

func NewDeleteSecurityAPIServiceResponse() (response *DeleteSecurityAPIServiceResponse) {
    response = &DeleteSecurityAPIServiceResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteSecurityAPIService
// 用于删除 API 服务。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteSecurityAPIService(c *Client, request *DeleteSecurityAPIServiceRequest) (response *DeleteSecurityAPIServiceResponse, err error) {
    return DeleteSecurityAPIServiceWithContext(context.Background(), c, request)
}

// DeleteSecurityAPIService
// 用于删除 API 服务。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteSecurityAPIServiceWithContext(ctx context.Context, c *Client, request *DeleteSecurityAPIServiceRequest) (response *DeleteSecurityAPIServiceResponse, err error) {
    if request == nil {
        request = NewDeleteSecurityAPIServiceRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteSecurityAPIService")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteSecurityAPIService require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteSecurityAPIServiceResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteSecurityClientAttesterRequest() (request *DeleteSecurityClientAttesterRequest) {
    request = &DeleteSecurityClientAttesterRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteSecurityClientAttester")
    
    
    return
}

func NewDeleteSecurityClientAttesterResponse() (response *DeleteSecurityClientAttesterResponse) {
    response = &DeleteSecurityClientAttesterResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteSecurityClientAttester
// 删除客户端认证选项。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteSecurityClientAttester(c *Client, request *DeleteSecurityClientAttesterRequest) (response *DeleteSecurityClientAttesterResponse, err error) {
    return DeleteSecurityClientAttesterWithContext(context.Background(), c, request)
}

// DeleteSecurityClientAttester
// 删除客户端认证选项。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteSecurityClientAttesterWithContext(ctx context.Context, c *Client, request *DeleteSecurityClientAttesterRequest) (response *DeleteSecurityClientAttesterResponse, err error) {
    if request == nil {
        request = NewDeleteSecurityClientAttesterRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteSecurityClientAttester")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteSecurityClientAttester require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteSecurityClientAttesterResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteSecurityIPGroupRequest() (request *DeleteSecurityIPGroupRequest) {
    request = &DeleteSecurityIPGroupRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteSecurityIPGroup")
    
    
    return
}

func NewDeleteSecurityIPGroupResponse() (response *DeleteSecurityIPGroupResponse) {
    response = &DeleteSecurityIPGroupResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteSecurityIPGroup
// 删除指定 IP 组，如果有规则引用了 IP 组情况，则不允许删除。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteSecurityIPGroup(c *Client, request *DeleteSecurityIPGroupRequest) (response *DeleteSecurityIPGroupResponse, err error) {
    return DeleteSecurityIPGroupWithContext(context.Background(), c, request)
}

// DeleteSecurityIPGroup
// 删除指定 IP 组，如果有规则引用了 IP 组情况，则不允许删除。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteSecurityIPGroupWithContext(ctx context.Context, c *Client, request *DeleteSecurityIPGroupRequest) (response *DeleteSecurityIPGroupResponse, err error) {
    if request == nil {
        request = NewDeleteSecurityIPGroupRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteSecurityIPGroup")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteSecurityIPGroup require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteSecurityIPGroupResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteSecurityJSInjectionRuleRequest() (request *DeleteSecurityJSInjectionRuleRequest) {
    request = &DeleteSecurityJSInjectionRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteSecurityJSInjectionRule")
    
    
    return
}

func NewDeleteSecurityJSInjectionRuleResponse() (response *DeleteSecurityJSInjectionRuleResponse) {
    response = &DeleteSecurityJSInjectionRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteSecurityJSInjectionRule
// 删除 JavaScript 注入规则。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteSecurityJSInjectionRule(c *Client, request *DeleteSecurityJSInjectionRuleRequest) (response *DeleteSecurityJSInjectionRuleResponse, err error) {
    return DeleteSecurityJSInjectionRuleWithContext(context.Background(), c, request)
}

// DeleteSecurityJSInjectionRule
// 删除 JavaScript 注入规则。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteSecurityJSInjectionRuleWithContext(ctx context.Context, c *Client, request *DeleteSecurityJSInjectionRuleRequest) (response *DeleteSecurityJSInjectionRuleResponse, err error) {
    if request == nil {
        request = NewDeleteSecurityJSInjectionRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteSecurityJSInjectionRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteSecurityJSInjectionRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteSecurityJSInjectionRuleResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteSharedCNAMERequest() (request *DeleteSharedCNAMERequest) {
    request = &DeleteSharedCNAMERequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteSharedCNAME")
    
    
    return
}

func NewDeleteSharedCNAMEResponse() (response *DeleteSharedCNAMEResponse) {
    response = &DeleteSharedCNAMEResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteSharedCNAME
// 用于删除共享 CNAME，该功能白名单内测中。
//
// 可能返回的错误码:
//  RESOURCEINUSE_SHAREDCNAME = "ResourceInUse.SharedCNAME"
func DeleteSharedCNAME(c *Client, request *DeleteSharedCNAMERequest) (response *DeleteSharedCNAMEResponse, err error) {
    return DeleteSharedCNAMEWithContext(context.Background(), c, request)
}

// DeleteSharedCNAME
// 用于删除共享 CNAME，该功能白名单内测中。
//
// 可能返回的错误码:
//  RESOURCEINUSE_SHAREDCNAME = "ResourceInUse.SharedCNAME"
func DeleteSharedCNAMEWithContext(ctx context.Context, c *Client, request *DeleteSharedCNAMERequest) (response *DeleteSharedCNAMEResponse, err error) {
    if request == nil {
        request = NewDeleteSharedCNAMERequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteSharedCNAME")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteSharedCNAME require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteSharedCNAMEResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteWebSecurityTemplateRequest() (request *DeleteWebSecurityTemplateRequest) {
    request = &DeleteWebSecurityTemplateRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteWebSecurityTemplate")
    
    
    return
}

func NewDeleteWebSecurityTemplateResponse() (response *DeleteWebSecurityTemplateResponse) {
    response = &DeleteWebSecurityTemplateResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteWebSecurityTemplate
// 删除安全策略配置模板
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func DeleteWebSecurityTemplate(c *Client, request *DeleteWebSecurityTemplateRequest) (response *DeleteWebSecurityTemplateResponse, err error) {
    return DeleteWebSecurityTemplateWithContext(context.Background(), c, request)
}

// DeleteWebSecurityTemplate
// 删除安全策略配置模板
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func DeleteWebSecurityTemplateWithContext(ctx context.Context, c *Client, request *DeleteWebSecurityTemplateRequest) (response *DeleteWebSecurityTemplateResponse, err error) {
    if request == nil {
        request = NewDeleteWebSecurityTemplateRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteWebSecurityTemplate")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteWebSecurityTemplate require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteWebSecurityTemplateResponse()
    err = c.Send(request, response)
    return
}

func NewDeleteZoneRequest() (request *DeleteZoneRequest) {
    request = &DeleteZoneRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeleteZone")
    
    
    return
}

func NewDeleteZoneResponse() (response *DeleteZoneResponse) {
    response = &DeleteZoneResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeleteZone
// 删除站点。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DELETEZONEPRECHECKFAILED = "OperationDenied.DeleteZonePreCheckFailed"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ZONEISBINDINGSHAREDCNAME = "OperationDenied.ZoneIsBindingSharedCNAME"
//  OPERATIONDENIED_ZONEISREFERENCECUSTOMERRORPAGE = "OperationDenied.ZoneIsReferenceCustomErrorPage"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteZone(c *Client, request *DeleteZoneRequest) (response *DeleteZoneResponse, err error) {
    return DeleteZoneWithContext(context.Background(), c, request)
}

// DeleteZone
// 删除站点。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DELETEZONEPRECHECKFAILED = "OperationDenied.DeleteZonePreCheckFailed"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ZONEISBINDINGSHAREDCNAME = "OperationDenied.ZoneIsBindingSharedCNAME"
//  OPERATIONDENIED_ZONEISREFERENCECUSTOMERRORPAGE = "OperationDenied.ZoneIsReferenceCustomErrorPage"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DeleteZoneWithContext(ctx context.Context, c *Client, request *DeleteZoneRequest) (response *DeleteZoneResponse, err error) {
    if request == nil {
        request = NewDeleteZoneRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeleteZone")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeleteZone require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeleteZoneResponse()
    err = c.Send(request, response)
    return
}

func NewDeployConfigGroupVersionRequest() (request *DeployConfigGroupVersionRequest) {
    request = &DeployConfigGroupVersionRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DeployConfigGroupVersion")
    
    
    return
}

func NewDeployConfigGroupVersionResponse() (response *DeployConfigGroupVersionResponse) {
    response = &DeployConfigGroupVersionResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DeployConfigGroupVersion
// 在版本管理模式下，用于版本发布，可通过 EnvId 将版本发布至测试环境或生产环境。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  FAILEDOPERATION_CONFIGCONDITIONSYNTAXERROR = "FailedOperation.ConfigConditionSyntaxError"
//  FAILEDOPERATION_CONFIGCONDITIONUNKNOWNTARGET = "FailedOperation.ConfigConditionUnknownTarget"
//  FAILEDOPERATION_CONFIGCONDITIONVALUEEMPTYERROR = "FailedOperation.ConfigConditionValueEmptyError"
//  FAILEDOPERATION_CONFIGFIELDTYPEERROR = "FailedOperation.ConfigFieldTypeError"
//  FAILEDOPERATION_CONFIGFORMATERROR = "FailedOperation.ConfigFormatError"
//  FAILEDOPERATION_CONFIGMALFORMEDCONTENT = "FailedOperation.ConfigMalformedContent"
//  FAILEDOPERATION_CONFIGPARAMVALIDATEERRORS = "FailedOperation.ConfigParamValidateErrors"
//  FAILEDOPERATION_CONFIGUNKNOWNFIELD = "FailedOperation.ConfigUnknownField"
//  FAILEDOPERATION_CONFIGUNSUPPORTEDFORMATVERSION = "FailedOperation.ConfigUnsupportedFormatVersion"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DOMAINSTATUSUNSTABLE = "OperationDenied.DomainStatusUnstable"
//  OPERATIONDENIED_ENVNOTREADY = "OperationDenied.EnvNotReady"
//  OPERATIONDENIED_NOTINVERSIONCONTROLWHITELIST = "OperationDenied.NotInVersionControlWhiteList"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  OPERATIONDENIED_WORKMODENOTINVERSIONCONTROL = "OperationDenied.WorkModeNotInVersionControl"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCENOTFOUND_VERSIONNOTFOUND = "ResourceNotFound.VersionNotFound"
func DeployConfigGroupVersion(c *Client, request *DeployConfigGroupVersionRequest) (response *DeployConfigGroupVersionResponse, err error) {
    return DeployConfigGroupVersionWithContext(context.Background(), c, request)
}

// DeployConfigGroupVersion
// 在版本管理模式下，用于版本发布，可通过 EnvId 将版本发布至测试环境或生产环境。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  FAILEDOPERATION_CONFIGCONDITIONSYNTAXERROR = "FailedOperation.ConfigConditionSyntaxError"
//  FAILEDOPERATION_CONFIGCONDITIONUNKNOWNTARGET = "FailedOperation.ConfigConditionUnknownTarget"
//  FAILEDOPERATION_CONFIGCONDITIONVALUEEMPTYERROR = "FailedOperation.ConfigConditionValueEmptyError"
//  FAILEDOPERATION_CONFIGFIELDTYPEERROR = "FailedOperation.ConfigFieldTypeError"
//  FAILEDOPERATION_CONFIGFORMATERROR = "FailedOperation.ConfigFormatError"
//  FAILEDOPERATION_CONFIGMALFORMEDCONTENT = "FailedOperation.ConfigMalformedContent"
//  FAILEDOPERATION_CONFIGPARAMVALIDATEERRORS = "FailedOperation.ConfigParamValidateErrors"
//  FAILEDOPERATION_CONFIGUNKNOWNFIELD = "FailedOperation.ConfigUnknownField"
//  FAILEDOPERATION_CONFIGUNSUPPORTEDFORMATVERSION = "FailedOperation.ConfigUnsupportedFormatVersion"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DOMAINSTATUSUNSTABLE = "OperationDenied.DomainStatusUnstable"
//  OPERATIONDENIED_ENVNOTREADY = "OperationDenied.EnvNotReady"
//  OPERATIONDENIED_NOTINVERSIONCONTROLWHITELIST = "OperationDenied.NotInVersionControlWhiteList"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  OPERATIONDENIED_WORKMODENOTINVERSIONCONTROL = "OperationDenied.WorkModeNotInVersionControl"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCENOTFOUND_VERSIONNOTFOUND = "ResourceNotFound.VersionNotFound"
func DeployConfigGroupVersionWithContext(ctx context.Context, c *Client, request *DeployConfigGroupVersionRequest) (response *DeployConfigGroupVersionResponse, err error) {
    if request == nil {
        request = NewDeployConfigGroupVersionRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DeployConfigGroupVersion")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DeployConfigGroupVersion require credential")
    }

    request.SetContext(ctx)
    
    response = NewDeployConfigGroupVersionResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeAccelerationDomainsRequest() (request *DescribeAccelerationDomainsRequest) {
    request = &DescribeAccelerationDomainsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeAccelerationDomains")
    
    
    return
}

func NewDescribeAccelerationDomainsResponse() (response *DescribeAccelerationDomainsResponse) {
    response = &DescribeAccelerationDomainsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeAccelerationDomains
// 您可以通过本接口查看站点下的域名信息，包括加速域名、源站以及域名状态等信息。您可以查看站点下全部域名的信息，也可以指定过滤条件查询对应的域名信息。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_DOMAINONTRAFFICSCHEDULING = "InvalidParameter.DomainOnTrafficScheduling"
//  INVALIDPARAMETER_TOOMANYFILTERVALUES = "InvalidParameter.TooManyFilterValues"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeAccelerationDomains(c *Client, request *DescribeAccelerationDomainsRequest) (response *DescribeAccelerationDomainsResponse, err error) {
    return DescribeAccelerationDomainsWithContext(context.Background(), c, request)
}

// DescribeAccelerationDomains
// 您可以通过本接口查看站点下的域名信息，包括加速域名、源站以及域名状态等信息。您可以查看站点下全部域名的信息，也可以指定过滤条件查询对应的域名信息。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_DOMAINONTRAFFICSCHEDULING = "InvalidParameter.DomainOnTrafficScheduling"
//  INVALIDPARAMETER_TOOMANYFILTERVALUES = "InvalidParameter.TooManyFilterValues"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeAccelerationDomainsWithContext(ctx context.Context, c *Client, request *DescribeAccelerationDomainsRequest) (response *DescribeAccelerationDomainsResponse, err error) {
    if request == nil {
        request = NewDescribeAccelerationDomainsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeAccelerationDomains")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeAccelerationDomains require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeAccelerationDomainsResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeAliasDomainsRequest() (request *DescribeAliasDomainsRequest) {
    request = &DescribeAliasDomainsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeAliasDomains")
    
    
    return
}

func NewDescribeAliasDomainsResponse() (response *DescribeAliasDomainsResponse) {
    response = &DescribeAliasDomainsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeAliasDomains
// 查询别称域名信息列表。
//
// 该功能仅企业版套餐支持，并且该功能当前仍在内测中，如需使用，请[联系我们](https://cloud.tencent.com/online-service?from=connect-us)。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeAliasDomains(c *Client, request *DescribeAliasDomainsRequest) (response *DescribeAliasDomainsResponse, err error) {
    return DescribeAliasDomainsWithContext(context.Background(), c, request)
}

// DescribeAliasDomains
// 查询别称域名信息列表。
//
// 该功能仅企业版套餐支持，并且该功能当前仍在内测中，如需使用，请[联系我们](https://cloud.tencent.com/online-service?from=connect-us)。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeAliasDomainsWithContext(ctx context.Context, c *Client, request *DescribeAliasDomainsRequest) (response *DescribeAliasDomainsResponse, err error) {
    if request == nil {
        request = NewDescribeAliasDomainsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeAliasDomains")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeAliasDomains require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeAliasDomainsResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeApplicationProxiesRequest() (request *DescribeApplicationProxiesRequest) {
    request = &DescribeApplicationProxiesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeApplicationProxies")
    
    
    return
}

func NewDescribeApplicationProxiesResponse() (response *DescribeApplicationProxiesResponse) {
    response = &DescribeApplicationProxiesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeApplicationProxies
// 本接口为旧版，如需调用请尽快迁移至新版，新版接口中将四层代理实例列表的查询和四层转发规则的查询拆分成两个接口，详情请参考 [查询四层代理实例列表](https://cloud.tencent.com/document/product/1552/103413) 和 [查询四层代理转发规则列表](https://cloud.tencent.com/document/product/1552/103412)。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeApplicationProxies(c *Client, request *DescribeApplicationProxiesRequest) (response *DescribeApplicationProxiesResponse, err error) {
    return DescribeApplicationProxiesWithContext(context.Background(), c, request)
}

// DescribeApplicationProxies
// 本接口为旧版，如需调用请尽快迁移至新版，新版接口中将四层代理实例列表的查询和四层转发规则的查询拆分成两个接口，详情请参考 [查询四层代理实例列表](https://cloud.tencent.com/document/product/1552/103413) 和 [查询四层代理转发规则列表](https://cloud.tencent.com/document/product/1552/103412)。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeApplicationProxiesWithContext(ctx context.Context, c *Client, request *DescribeApplicationProxiesRequest) (response *DescribeApplicationProxiesResponse, err error) {
    if request == nil {
        request = NewDescribeApplicationProxiesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeApplicationProxies")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeApplicationProxies require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeApplicationProxiesResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeAvailablePlansRequest() (request *DescribeAvailablePlansRequest) {
    request = &DescribeAvailablePlansRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeAvailablePlans")
    
    
    return
}

func NewDescribeAvailablePlansResponse() (response *DescribeAvailablePlansResponse) {
    response = &DescribeAvailablePlansResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeAvailablePlans
// 查询当前账户可用套餐信息列表
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
func DescribeAvailablePlans(c *Client, request *DescribeAvailablePlansRequest) (response *DescribeAvailablePlansResponse, err error) {
    return DescribeAvailablePlansWithContext(context.Background(), c, request)
}

// DescribeAvailablePlans
// 查询当前账户可用套餐信息列表
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
func DescribeAvailablePlansWithContext(ctx context.Context, c *Client, request *DescribeAvailablePlansRequest) (response *DescribeAvailablePlansResponse, err error) {
    if request == nil {
        request = NewDescribeAvailablePlansRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeAvailablePlans")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeAvailablePlans require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeAvailablePlansResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeBillingDataRequest() (request *DescribeBillingDataRequest) {
    request = &DescribeBillingDataRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeBillingData")
    
    
    return
}

func NewDescribeBillingDataResponse() (response *DescribeBillingDataResponse) {
    response = &DescribeBillingDataResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeBillingData
// 通过本接口查询计费数据。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_GROUPBYLIMITEXCEEDED = "InvalidParameter.GroupByLimitExceeded"
//  INVALIDPARAMETER_INVALIDINTERVAL = "InvalidParameter.InvalidInterval"
//  INVALIDPARAMETER_INVALIDMETRIC = "InvalidParameter.InvalidMetric"
//  INVALIDPARAMETER_ZONEHASNOTBEENBOUNDTOPLAN = "InvalidParameter.ZoneHasNotBeenBoundToPlan"
func DescribeBillingData(c *Client, request *DescribeBillingDataRequest) (response *DescribeBillingDataResponse, err error) {
    return DescribeBillingDataWithContext(context.Background(), c, request)
}

// DescribeBillingData
// 通过本接口查询计费数据。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_GROUPBYLIMITEXCEEDED = "InvalidParameter.GroupByLimitExceeded"
//  INVALIDPARAMETER_INVALIDINTERVAL = "InvalidParameter.InvalidInterval"
//  INVALIDPARAMETER_INVALIDMETRIC = "InvalidParameter.InvalidMetric"
//  INVALIDPARAMETER_ZONEHASNOTBEENBOUNDTOPLAN = "InvalidParameter.ZoneHasNotBeenBoundToPlan"
func DescribeBillingDataWithContext(ctx context.Context, c *Client, request *DescribeBillingDataRequest) (response *DescribeBillingDataResponse, err error) {
    if request == nil {
        request = NewDescribeBillingDataRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeBillingData")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeBillingData require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeBillingDataResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeConfigGroupVersionDetailRequest() (request *DescribeConfigGroupVersionDetailRequest) {
    request = &DescribeConfigGroupVersionDetailRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeConfigGroupVersionDetail")
    
    
    return
}

func NewDescribeConfigGroupVersionDetailResponse() (response *DescribeConfigGroupVersionDetailResponse) {
    response = &DescribeConfigGroupVersionDetailResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeConfigGroupVersionDetail
// 在版本管理模式下，用于获取版本的详细信息，包括版本 ID、描述、状态、创建时间、所属配置组信息以及版本配置文件的内容。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeConfigGroupVersionDetail(c *Client, request *DescribeConfigGroupVersionDetailRequest) (response *DescribeConfigGroupVersionDetailResponse, err error) {
    return DescribeConfigGroupVersionDetailWithContext(context.Background(), c, request)
}

// DescribeConfigGroupVersionDetail
// 在版本管理模式下，用于获取版本的详细信息，包括版本 ID、描述、状态、创建时间、所属配置组信息以及版本配置文件的内容。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeConfigGroupVersionDetailWithContext(ctx context.Context, c *Client, request *DescribeConfigGroupVersionDetailRequest) (response *DescribeConfigGroupVersionDetailResponse, err error) {
    if request == nil {
        request = NewDescribeConfigGroupVersionDetailRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeConfigGroupVersionDetail")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeConfigGroupVersionDetail require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeConfigGroupVersionDetailResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeConfigGroupVersionsRequest() (request *DescribeConfigGroupVersionsRequest) {
    request = &DescribeConfigGroupVersionsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeConfigGroupVersions")
    
    
    return
}

func NewDescribeConfigGroupVersionsResponse() (response *DescribeConfigGroupVersionsResponse) {
    response = &DescribeConfigGroupVersionsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeConfigGroupVersions
// 在版本管理模式下，用于查询指定配置组的版本列表。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeConfigGroupVersions(c *Client, request *DescribeConfigGroupVersionsRequest) (response *DescribeConfigGroupVersionsResponse, err error) {
    return DescribeConfigGroupVersionsWithContext(context.Background(), c, request)
}

// DescribeConfigGroupVersions
// 在版本管理模式下，用于查询指定配置组的版本列表。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeConfigGroupVersionsWithContext(ctx context.Context, c *Client, request *DescribeConfigGroupVersionsRequest) (response *DescribeConfigGroupVersionsResponse, err error) {
    if request == nil {
        request = NewDescribeConfigGroupVersionsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeConfigGroupVersions")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeConfigGroupVersions require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeConfigGroupVersionsResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeContentIdentifiersRequest() (request *DescribeContentIdentifiersRequest) {
    request = &DescribeContentIdentifiersRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeContentIdentifiers")
    
    
    return
}

func NewDescribeContentIdentifiersResponse() (response *DescribeContentIdentifiersResponse) {
    response = &DescribeContentIdentifiersResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeContentIdentifiers
// 批量查询内容标识符，可以根据 ID、描述、状态或者标签过滤。按照状态查询被删除的内容标识符仅保留三个月。该功能仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeContentIdentifiers(c *Client, request *DescribeContentIdentifiersRequest) (response *DescribeContentIdentifiersResponse, err error) {
    return DescribeContentIdentifiersWithContext(context.Background(), c, request)
}

// DescribeContentIdentifiers
// 批量查询内容标识符，可以根据 ID、描述、状态或者标签过滤。按照状态查询被删除的内容标识符仅保留三个月。该功能仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeContentIdentifiersWithContext(ctx context.Context, c *Client, request *DescribeContentIdentifiersRequest) (response *DescribeContentIdentifiersResponse, err error) {
    if request == nil {
        request = NewDescribeContentIdentifiersRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeContentIdentifiers")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeContentIdentifiers require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeContentIdentifiersResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeContentQuotaRequest() (request *DescribeContentQuotaRequest) {
    request = &DescribeContentQuotaRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeContentQuota")
    
    
    return
}

func NewDescribeContentQuotaResponse() (response *DescribeContentQuotaResponse) {
    response = &DescribeContentQuotaResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeContentQuota
// 查询内容管理接口配额
//
// 可能返回的错误码:
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeContentQuota(c *Client, request *DescribeContentQuotaRequest) (response *DescribeContentQuotaResponse, err error) {
    return DescribeContentQuotaWithContext(context.Background(), c, request)
}

// DescribeContentQuota
// 查询内容管理接口配额
//
// 可能返回的错误码:
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeContentQuotaWithContext(ctx context.Context, c *Client, request *DescribeContentQuotaRequest) (response *DescribeContentQuotaResponse, err error) {
    if request == nil {
        request = NewDescribeContentQuotaRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeContentQuota")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeContentQuota require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeContentQuotaResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeCustomErrorPagesRequest() (request *DescribeCustomErrorPagesRequest) {
    request = &DescribeCustomErrorPagesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeCustomErrorPages")
    
    
    return
}

func NewDescribeCustomErrorPagesResponse() (response *DescribeCustomErrorPagesResponse) {
    response = &DescribeCustomErrorPagesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeCustomErrorPages
// 查询自定义错误页列表。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
func DescribeCustomErrorPages(c *Client, request *DescribeCustomErrorPagesRequest) (response *DescribeCustomErrorPagesResponse, err error) {
    return DescribeCustomErrorPagesWithContext(context.Background(), c, request)
}

// DescribeCustomErrorPages
// 查询自定义错误页列表。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
func DescribeCustomErrorPagesWithContext(ctx context.Context, c *Client, request *DescribeCustomErrorPagesRequest) (response *DescribeCustomErrorPagesResponse, err error) {
    if request == nil {
        request = NewDescribeCustomErrorPagesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeCustomErrorPages")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeCustomErrorPages require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeCustomErrorPagesResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeDDoSAttackDataRequest() (request *DescribeDDoSAttackDataRequest) {
    request = &DescribeDDoSAttackDataRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeDDoSAttackData")
    
    
    return
}

func NewDescribeDDoSAttackDataResponse() (response *DescribeDDoSAttackDataResponse) {
    response = &DescribeDDoSAttackDataResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeDDoSAttackData
// 本接口（DescribeDDoSAttackData）用于查询DDoS攻击时序数据。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func DescribeDDoSAttackData(c *Client, request *DescribeDDoSAttackDataRequest) (response *DescribeDDoSAttackDataResponse, err error) {
    return DescribeDDoSAttackDataWithContext(context.Background(), c, request)
}

// DescribeDDoSAttackData
// 本接口（DescribeDDoSAttackData）用于查询DDoS攻击时序数据。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func DescribeDDoSAttackDataWithContext(ctx context.Context, c *Client, request *DescribeDDoSAttackDataRequest) (response *DescribeDDoSAttackDataResponse, err error) {
    if request == nil {
        request = NewDescribeDDoSAttackDataRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeDDoSAttackData")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeDDoSAttackData require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeDDoSAttackDataResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeDDoSAttackEventRequest() (request *DescribeDDoSAttackEventRequest) {
    request = &DescribeDDoSAttackEventRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeDDoSAttackEvent")
    
    
    return
}

func NewDescribeDDoSAttackEventResponse() (response *DescribeDDoSAttackEventResponse) {
    response = &DescribeDDoSAttackEventResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeDDoSAttackEvent
// 本接口（DescribeDDoSAttackEvent）用于查询DDoS攻击事件列表。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeDDoSAttackEvent(c *Client, request *DescribeDDoSAttackEventRequest) (response *DescribeDDoSAttackEventResponse, err error) {
    return DescribeDDoSAttackEventWithContext(context.Background(), c, request)
}

// DescribeDDoSAttackEvent
// 本接口（DescribeDDoSAttackEvent）用于查询DDoS攻击事件列表。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeDDoSAttackEventWithContext(ctx context.Context, c *Client, request *DescribeDDoSAttackEventRequest) (response *DescribeDDoSAttackEventResponse, err error) {
    if request == nil {
        request = NewDescribeDDoSAttackEventRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeDDoSAttackEvent")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeDDoSAttackEvent require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeDDoSAttackEventResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeDDoSAttackTopDataRequest() (request *DescribeDDoSAttackTopDataRequest) {
    request = &DescribeDDoSAttackTopDataRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeDDoSAttackTopData")
    
    
    return
}

func NewDescribeDDoSAttackTopDataResponse() (response *DescribeDDoSAttackTopDataResponse) {
    response = &DescribeDDoSAttackTopDataResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeDDoSAttackTopData
// 本接口（DescribeDDoSAttackTopData）用于查询DDoS攻击Top数据。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeDDoSAttackTopData(c *Client, request *DescribeDDoSAttackTopDataRequest) (response *DescribeDDoSAttackTopDataResponse, err error) {
    return DescribeDDoSAttackTopDataWithContext(context.Background(), c, request)
}

// DescribeDDoSAttackTopData
// 本接口（DescribeDDoSAttackTopData）用于查询DDoS攻击Top数据。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeDDoSAttackTopDataWithContext(ctx context.Context, c *Client, request *DescribeDDoSAttackTopDataRequest) (response *DescribeDDoSAttackTopDataResponse, err error) {
    if request == nil {
        request = NewDescribeDDoSAttackTopDataRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeDDoSAttackTopData")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeDDoSAttackTopData require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeDDoSAttackTopDataResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeDDoSProtectionRequest() (request *DescribeDDoSProtectionRequest) {
    request = &DescribeDDoSProtectionRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeDDoSProtection")
    
    
    return
}

func NewDescribeDDoSProtectionResponse() (response *DescribeDDoSProtectionResponse) {
    response = &DescribeDDoSProtectionResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeDDoSProtection
// 获取站点的独立 DDoS 防护信息。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeDDoSProtection(c *Client, request *DescribeDDoSProtectionRequest) (response *DescribeDDoSProtectionResponse, err error) {
    return DescribeDDoSProtectionWithContext(context.Background(), c, request)
}

// DescribeDDoSProtection
// 获取站点的独立 DDoS 防护信息。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeDDoSProtectionWithContext(ctx context.Context, c *Client, request *DescribeDDoSProtectionRequest) (response *DescribeDDoSProtectionResponse, err error) {
    if request == nil {
        request = NewDescribeDDoSProtectionRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeDDoSProtection")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeDDoSProtection require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeDDoSProtectionResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeDefaultCertificatesRequest() (request *DescribeDefaultCertificatesRequest) {
    request = &DescribeDefaultCertificatesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeDefaultCertificates")
    
    
    return
}

func NewDescribeDefaultCertificatesResponse() (response *DescribeDefaultCertificatesResponse) {
    response = &DescribeDefaultCertificatesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeDefaultCertificates
// 查询默认证书列表
//
// 可能返回的错误码:
//  INTERNALERROR_DBERROR = "InternalError.DBError"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func DescribeDefaultCertificates(c *Client, request *DescribeDefaultCertificatesRequest) (response *DescribeDefaultCertificatesResponse, err error) {
    return DescribeDefaultCertificatesWithContext(context.Background(), c, request)
}

// DescribeDefaultCertificates
// 查询默认证书列表
//
// 可能返回的错误码:
//  INTERNALERROR_DBERROR = "InternalError.DBError"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func DescribeDefaultCertificatesWithContext(ctx context.Context, c *Client, request *DescribeDefaultCertificatesRequest) (response *DescribeDefaultCertificatesResponse, err error) {
    if request == nil {
        request = NewDescribeDefaultCertificatesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeDefaultCertificates")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeDefaultCertificates require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeDefaultCertificatesResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeDeployHistoryRequest() (request *DescribeDeployHistoryRequest) {
    request = &DescribeDeployHistoryRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeDeployHistory")
    
    
    return
}

func NewDescribeDeployHistoryResponse() (response *DescribeDeployHistoryResponse) {
    response = &DescribeDeployHistoryResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeDeployHistory
// 在版本管理模式下，用于查询生产/测试环境的版本发布历史。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeDeployHistory(c *Client, request *DescribeDeployHistoryRequest) (response *DescribeDeployHistoryResponse, err error) {
    return DescribeDeployHistoryWithContext(context.Background(), c, request)
}

// DescribeDeployHistory
// 在版本管理模式下，用于查询生产/测试环境的版本发布历史。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeDeployHistoryWithContext(ctx context.Context, c *Client, request *DescribeDeployHistoryRequest) (response *DescribeDeployHistoryResponse, err error) {
    if request == nil {
        request = NewDescribeDeployHistoryRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeDeployHistory")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeDeployHistory require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeDeployHistoryResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeDnsRecordsRequest() (request *DescribeDnsRecordsRequest) {
    request = &DescribeDnsRecordsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeDnsRecords")
    
    
    return
}

func NewDescribeDnsRecordsResponse() (response *DescribeDnsRecordsResponse) {
    response = &DescribeDnsRecordsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeDnsRecords
// 您可以用过本接口查看站点下的 DNS 记录信息，包括 DNS 记录名、记录类型以及记录内容等信息，支持指定过滤条件查询对应的 DNS 记录信息。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeDnsRecords(c *Client, request *DescribeDnsRecordsRequest) (response *DescribeDnsRecordsResponse, err error) {
    return DescribeDnsRecordsWithContext(context.Background(), c, request)
}

// DescribeDnsRecords
// 您可以用过本接口查看站点下的 DNS 记录信息，包括 DNS 记录名、记录类型以及记录内容等信息，支持指定过滤条件查询对应的 DNS 记录信息。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeDnsRecordsWithContext(ctx context.Context, c *Client, request *DescribeDnsRecordsRequest) (response *DescribeDnsRecordsResponse, err error) {
    if request == nil {
        request = NewDescribeDnsRecordsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeDnsRecords")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeDnsRecords require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeDnsRecordsResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeEnvironmentsRequest() (request *DescribeEnvironmentsRequest) {
    request = &DescribeEnvironmentsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeEnvironments")
    
    
    return
}

func NewDescribeEnvironmentsResponse() (response *DescribeEnvironmentsResponse) {
    response = &DescribeEnvironmentsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeEnvironments
// 在版本管理模式下，用于查询环境信息，可获取环境 ID、类型、当前生效版本等。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeEnvironments(c *Client, request *DescribeEnvironmentsRequest) (response *DescribeEnvironmentsResponse, err error) {
    return DescribeEnvironmentsWithContext(context.Background(), c, request)
}

// DescribeEnvironments
// 在版本管理模式下，用于查询环境信息，可获取环境 ID、类型、当前生效版本等。版本管理功能内测中，当前仅白名单开放。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func DescribeEnvironmentsWithContext(ctx context.Context, c *Client, request *DescribeEnvironmentsRequest) (response *DescribeEnvironmentsResponse, err error) {
    if request == nil {
        request = NewDescribeEnvironmentsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeEnvironments")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeEnvironments require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeEnvironmentsResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeFunctionRulesRequest() (request *DescribeFunctionRulesRequest) {
    request = &DescribeFunctionRulesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeFunctionRules")
    
    
    return
}

func NewDescribeFunctionRulesResponse() (response *DescribeFunctionRulesResponse) {
    response = &DescribeFunctionRulesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeFunctionRules
// 查询边缘函数触发规则列表，支持按照规则 ID、函数 ID、规则描述等条件进行过滤。
//
// 可能返回的错误码:
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_INVALIDFILTERNAME = "InvalidParameter.InvalidFilterName"
//  INVALIDPARAMETER_TOOMANYFILTERVALUES = "InvalidParameter.TooManyFilterValues"
//  INVALIDPARAMETER_TOOMANYFILTERS = "InvalidParameter.TooManyFilters"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func DescribeFunctionRules(c *Client, request *DescribeFunctionRulesRequest) (response *DescribeFunctionRulesResponse, err error) {
    return DescribeFunctionRulesWithContext(context.Background(), c, request)
}

// DescribeFunctionRules
// 查询边缘函数触发规则列表，支持按照规则 ID、函数 ID、规则描述等条件进行过滤。
//
// 可能返回的错误码:
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_INVALIDFILTERNAME = "InvalidParameter.InvalidFilterName"
//  INVALIDPARAMETER_TOOMANYFILTERVALUES = "InvalidParameter.TooManyFilterValues"
//  INVALIDPARAMETER_TOOMANYFILTERS = "InvalidParameter.TooManyFilters"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func DescribeFunctionRulesWithContext(ctx context.Context, c *Client, request *DescribeFunctionRulesRequest) (response *DescribeFunctionRulesResponse, err error) {
    if request == nil {
        request = NewDescribeFunctionRulesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeFunctionRules")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeFunctionRules require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeFunctionRulesResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeFunctionRuntimeEnvironmentRequest() (request *DescribeFunctionRuntimeEnvironmentRequest) {
    request = &DescribeFunctionRuntimeEnvironmentRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeFunctionRuntimeEnvironment")
    
    
    return
}

func NewDescribeFunctionRuntimeEnvironmentResponse() (response *DescribeFunctionRuntimeEnvironmentResponse) {
    response = &DescribeFunctionRuntimeEnvironmentResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeFunctionRuntimeEnvironment
// 查询边缘函数运行环境，包括环境变量。
//
// 可能返回的错误码:
//  INTERNALERROR_DBERROR = "InternalError.DBError"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_INVALIDFILTERNAME = "InvalidParameter.InvalidFilterName"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func DescribeFunctionRuntimeEnvironment(c *Client, request *DescribeFunctionRuntimeEnvironmentRequest) (response *DescribeFunctionRuntimeEnvironmentResponse, err error) {
    return DescribeFunctionRuntimeEnvironmentWithContext(context.Background(), c, request)
}

// DescribeFunctionRuntimeEnvironment
// 查询边缘函数运行环境，包括环境变量。
//
// 可能返回的错误码:
//  INTERNALERROR_DBERROR = "InternalError.DBError"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_INVALIDFILTERNAME = "InvalidParameter.InvalidFilterName"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func DescribeFunctionRuntimeEnvironmentWithContext(ctx context.Context, c *Client, request *DescribeFunctionRuntimeEnvironmentRequest) (response *DescribeFunctionRuntimeEnvironmentResponse, err error) {
    if request == nil {
        request = NewDescribeFunctionRuntimeEnvironmentRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeFunctionRuntimeEnvironment")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeFunctionRuntimeEnvironment require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeFunctionRuntimeEnvironmentResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeFunctionsRequest() (request *DescribeFunctionsRequest) {
    request = &DescribeFunctionsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeFunctions")
    
    
    return
}

func NewDescribeFunctionsResponse() (response *DescribeFunctionsResponse) {
    response = &DescribeFunctionsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeFunctions
// 查询边缘函数列表，支持函数 ID、函数名称、描述等条件的过滤。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_INVALIDFILTERNAME = "InvalidParameter.InvalidFilterName"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  INVALIDPARAMETER_TOOMANYFILTERVALUES = "InvalidParameter.TooManyFilterValues"
//  INVALIDPARAMETER_TOOMANYFILTERS = "InvalidParameter.TooManyFilters"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func DescribeFunctions(c *Client, request *DescribeFunctionsRequest) (response *DescribeFunctionsResponse, err error) {
    return DescribeFunctionsWithContext(context.Background(), c, request)
}

// DescribeFunctions
// 查询边缘函数列表，支持函数 ID、函数名称、描述等条件的过滤。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_INVALIDFILTERNAME = "InvalidParameter.InvalidFilterName"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  INVALIDPARAMETER_TOOMANYFILTERVALUES = "InvalidParameter.TooManyFilterValues"
//  INVALIDPARAMETER_TOOMANYFILTERS = "InvalidParameter.TooManyFilters"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func DescribeFunctionsWithContext(ctx context.Context, c *Client, request *DescribeFunctionsRequest) (response *DescribeFunctionsResponse, err error) {
    if request == nil {
        request = NewDescribeFunctionsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeFunctions")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeFunctions require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeFunctionsResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeHostsSettingRequest() (request *DescribeHostsSettingRequest) {
    request = &DescribeHostsSettingRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeHostsSetting")
    
    
    return
}

func NewDescribeHostsSettingResponse() (response *DescribeHostsSettingResponse) {
    response = &DescribeHostsSettingResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeHostsSetting
// 本接口为旧版，EdgeOne 已对规则引擎相关接口全面升级，可通过 [DescribeL7AccSetting](https://cloud.tencent.com/document/product/1552/115819) 和 [DescribeL7AccRules](https://cloud.tencent.com/document/product/1552/115820) 来获取域名的详细配置。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INVALIDPARAMETER = "InvalidParameter"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeHostsSetting(c *Client, request *DescribeHostsSettingRequest) (response *DescribeHostsSettingResponse, err error) {
    return DescribeHostsSettingWithContext(context.Background(), c, request)
}

// DescribeHostsSetting
// 本接口为旧版，EdgeOne 已对规则引擎相关接口全面升级，可通过 [DescribeL7AccSetting](https://cloud.tencent.com/document/product/1552/115819) 和 [DescribeL7AccRules](https://cloud.tencent.com/document/product/1552/115820) 来获取域名的详细配置。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INVALIDPARAMETER = "InvalidParameter"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeHostsSettingWithContext(ctx context.Context, c *Client, request *DescribeHostsSettingRequest) (response *DescribeHostsSettingResponse, err error) {
    if request == nil {
        request = NewDescribeHostsSettingRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeHostsSetting")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeHostsSetting require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeHostsSettingResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeIPRegionRequest() (request *DescribeIPRegionRequest) {
    request = &DescribeIPRegionRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeIPRegion")
    
    
    return
}

func NewDescribeIPRegionResponse() (response *DescribeIPRegionResponse) {
    response = &DescribeIPRegionResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeIPRegion
// 该接口可用于查询 IP 是否为 EdgeOne IP。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
func DescribeIPRegion(c *Client, request *DescribeIPRegionRequest) (response *DescribeIPRegionResponse, err error) {
    return DescribeIPRegionWithContext(context.Background(), c, request)
}

// DescribeIPRegion
// 该接口可用于查询 IP 是否为 EdgeOne IP。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
func DescribeIPRegionWithContext(ctx context.Context, c *Client, request *DescribeIPRegionRequest) (response *DescribeIPRegionResponse, err error) {
    if request == nil {
        request = NewDescribeIPRegionRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeIPRegion")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeIPRegion require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeIPRegionResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeIdentificationsRequest() (request *DescribeIdentificationsRequest) {
    request = &DescribeIdentificationsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeIdentifications")
    
    
    return
}

func NewDescribeIdentificationsResponse() (response *DescribeIdentificationsResponse) {
    response = &DescribeIdentificationsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeIdentifications
// 查询站点的验证信息。
//
// 可能返回的错误码:
//  OPERATIONDENIED_DOMAININSHARECNAMEGROUP = "OperationDenied.DomainInShareCnameGroup"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
func DescribeIdentifications(c *Client, request *DescribeIdentificationsRequest) (response *DescribeIdentificationsResponse, err error) {
    return DescribeIdentificationsWithContext(context.Background(), c, request)
}

// DescribeIdentifications
// 查询站点的验证信息。
//
// 可能返回的错误码:
//  OPERATIONDENIED_DOMAININSHARECNAMEGROUP = "OperationDenied.DomainInShareCnameGroup"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
func DescribeIdentificationsWithContext(ctx context.Context, c *Client, request *DescribeIdentificationsRequest) (response *DescribeIdentificationsResponse, err error) {
    if request == nil {
        request = NewDescribeIdentificationsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeIdentifications")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeIdentifications require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeIdentificationsResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeJustInTimeTranscodeTemplatesRequest() (request *DescribeJustInTimeTranscodeTemplatesRequest) {
    request = &DescribeJustInTimeTranscodeTemplatesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeJustInTimeTranscodeTemplates")
    
    
    return
}

func NewDescribeJustInTimeTranscodeTemplatesResponse() (response *DescribeJustInTimeTranscodeTemplatesResponse) {
    response = &DescribeJustInTimeTranscodeTemplatesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeJustInTimeTranscodeTemplates
// 根据即时转码模板名字、模板类型或唯一标识，获取即时转码模板详情列表。返回结果包含符合条件的所有用户自定义模板及预置模板。
//
// 可能返回的错误码:
//  FAILEDOPERATION_PRODUCTDISCONTINUED = "FailedOperation.ProductDiscontinued"
//  FAILEDOPERATION_PRODUCTNOTACTIVATED = "FailedOperation.ProductNotActivated"
//  INTERNALERROR = "InternalError"
//  INVALIDFILTER = "InvalidFilter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_ZONENOTFOUND = "InvalidParameterValue.ZoneNotFound"
//  INVALIDPARAMETERVALUE_ZONEPAUSED = "InvalidParameterValue.ZonePaused"
//  MISSINGPARAMETER = "MissingParameter"
//  UNAUTHORIZEDOPERATION = "UnauthorizedOperation"
func DescribeJustInTimeTranscodeTemplates(c *Client, request *DescribeJustInTimeTranscodeTemplatesRequest) (response *DescribeJustInTimeTranscodeTemplatesResponse, err error) {
    return DescribeJustInTimeTranscodeTemplatesWithContext(context.Background(), c, request)
}

// DescribeJustInTimeTranscodeTemplates
// 根据即时转码模板名字、模板类型或唯一标识，获取即时转码模板详情列表。返回结果包含符合条件的所有用户自定义模板及预置模板。
//
// 可能返回的错误码:
//  FAILEDOPERATION_PRODUCTDISCONTINUED = "FailedOperation.ProductDiscontinued"
//  FAILEDOPERATION_PRODUCTNOTACTIVATED = "FailedOperation.ProductNotActivated"
//  INTERNALERROR = "InternalError"
//  INVALIDFILTER = "InvalidFilter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_ZONENOTFOUND = "InvalidParameterValue.ZoneNotFound"
//  INVALIDPARAMETERVALUE_ZONEPAUSED = "InvalidParameterValue.ZonePaused"
//  MISSINGPARAMETER = "MissingParameter"
//  UNAUTHORIZEDOPERATION = "UnauthorizedOperation"
func DescribeJustInTimeTranscodeTemplatesWithContext(ctx context.Context, c *Client, request *DescribeJustInTimeTranscodeTemplatesRequest) (response *DescribeJustInTimeTranscodeTemplatesResponse, err error) {
    if request == nil {
        request = NewDescribeJustInTimeTranscodeTemplatesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeJustInTimeTranscodeTemplates")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeJustInTimeTranscodeTemplates require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeJustInTimeTranscodeTemplatesResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeL4ProxyRequest() (request *DescribeL4ProxyRequest) {
    request = &DescribeL4ProxyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeL4Proxy")
    
    
    return
}

func NewDescribeL4ProxyResponse() (response *DescribeL4ProxyResponse) {
    response = &DescribeL4ProxyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeL4Proxy
// 用于查询四层代理实例列表。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeL4Proxy(c *Client, request *DescribeL4ProxyRequest) (response *DescribeL4ProxyResponse, err error) {
    return DescribeL4ProxyWithContext(context.Background(), c, request)
}

// DescribeL4Proxy
// 用于查询四层代理实例列表。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeL4ProxyWithContext(ctx context.Context, c *Client, request *DescribeL4ProxyRequest) (response *DescribeL4ProxyResponse, err error) {
    if request == nil {
        request = NewDescribeL4ProxyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeL4Proxy")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeL4Proxy require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeL4ProxyResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeL4ProxyRulesRequest() (request *DescribeL4ProxyRulesRequest) {
    request = &DescribeL4ProxyRulesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeL4ProxyRules")
    
    
    return
}

func NewDescribeL4ProxyRulesResponse() (response *DescribeL4ProxyRulesResponse) {
    response = &DescribeL4ProxyRulesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeL4ProxyRules
// 查询四层代理实例下的转发规则列表。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeL4ProxyRules(c *Client, request *DescribeL4ProxyRulesRequest) (response *DescribeL4ProxyRulesResponse, err error) {
    return DescribeL4ProxyRulesWithContext(context.Background(), c, request)
}

// DescribeL4ProxyRules
// 查询四层代理实例下的转发规则列表。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeL4ProxyRulesWithContext(ctx context.Context, c *Client, request *DescribeL4ProxyRulesRequest) (response *DescribeL4ProxyRulesResponse, err error) {
    if request == nil {
        request = NewDescribeL4ProxyRulesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeL4ProxyRules")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeL4ProxyRules require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeL4ProxyRulesResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeL7AccRulesRequest() (request *DescribeL7AccRulesRequest) {
    request = &DescribeL7AccRulesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeL7AccRules")
    
    
    return
}

func NewDescribeL7AccRulesResponse() (response *DescribeL7AccRulesResponse) {
    response = &DescribeL7AccRulesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeL7AccRules
// 本接口用于查询[规则引擎](https://cloud.tencent.com/document/product/1552/70901)的规则列表。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeL7AccRules(c *Client, request *DescribeL7AccRulesRequest) (response *DescribeL7AccRulesResponse, err error) {
    return DescribeL7AccRulesWithContext(context.Background(), c, request)
}

// DescribeL7AccRules
// 本接口用于查询[规则引擎](https://cloud.tencent.com/document/product/1552/70901)的规则列表。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeL7AccRulesWithContext(ctx context.Context, c *Client, request *DescribeL7AccRulesRequest) (response *DescribeL7AccRulesResponse, err error) {
    if request == nil {
        request = NewDescribeL7AccRulesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeL7AccRules")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeL7AccRules require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeL7AccRulesResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeL7AccSettingRequest() (request *DescribeL7AccSettingRequest) {
    request = &DescribeL7AccSettingRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeL7AccSetting")
    
    
    return
}

func NewDescribeL7AccSettingResponse() (response *DescribeL7AccSettingResponse) {
    response = &DescribeL7AccSettingResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeL7AccSetting
// 本接口用于查询[站点加速](https://cloud.tencent.com/document/product/1552/96193)全局配置。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_SETTINGINVALIDPARAM = "InvalidParameter.SettingInvalidParam"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeL7AccSetting(c *Client, request *DescribeL7AccSettingRequest) (response *DescribeL7AccSettingResponse, err error) {
    return DescribeL7AccSettingWithContext(context.Background(), c, request)
}

// DescribeL7AccSetting
// 本接口用于查询[站点加速](https://cloud.tencent.com/document/product/1552/96193)全局配置。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_SETTINGINVALIDPARAM = "InvalidParameter.SettingInvalidParam"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeL7AccSettingWithContext(ctx context.Context, c *Client, request *DescribeL7AccSettingRequest) (response *DescribeL7AccSettingResponse, err error) {
    if request == nil {
        request = NewDescribeL7AccSettingRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeL7AccSetting")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeL7AccSetting require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeL7AccSettingResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeLoadBalancerListRequest() (request *DescribeLoadBalancerListRequest) {
    request = &DescribeLoadBalancerListRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeLoadBalancerList")
    
    
    return
}

func NewDescribeLoadBalancerListResponse() (response *DescribeLoadBalancerListResponse) {
    response = &DescribeLoadBalancerListResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeLoadBalancerList
// 查询负载均衡实例列表。负载均衡功能内测中，如您需要使用请 [联系我们](https://cloud.tencent.com/online-service)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
func DescribeLoadBalancerList(c *Client, request *DescribeLoadBalancerListRequest) (response *DescribeLoadBalancerListResponse, err error) {
    return DescribeLoadBalancerListWithContext(context.Background(), c, request)
}

// DescribeLoadBalancerList
// 查询负载均衡实例列表。负载均衡功能内测中，如您需要使用请 [联系我们](https://cloud.tencent.com/online-service)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
func DescribeLoadBalancerListWithContext(ctx context.Context, c *Client, request *DescribeLoadBalancerListRequest) (response *DescribeLoadBalancerListResponse, err error) {
    if request == nil {
        request = NewDescribeLoadBalancerListRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeLoadBalancerList")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeLoadBalancerList require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeLoadBalancerListResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeMultiPathGatewayRequest() (request *DescribeMultiPathGatewayRequest) {
    request = &DescribeMultiPathGatewayRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeMultiPathGateway")
    
    
    return
}

func NewDescribeMultiPathGatewayResponse() (response *DescribeMultiPathGatewayResponse) {
    response = &DescribeMultiPathGatewayResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeMultiPathGateway
// 通过本接口查询多通道安全加速网关详情。如名称、网关 ID、IP、端口、类型等。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DescribeMultiPathGateway(c *Client, request *DescribeMultiPathGatewayRequest) (response *DescribeMultiPathGatewayResponse, err error) {
    return DescribeMultiPathGatewayWithContext(context.Background(), c, request)
}

// DescribeMultiPathGateway
// 通过本接口查询多通道安全加速网关详情。如名称、网关 ID、IP、端口、类型等。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DescribeMultiPathGatewayWithContext(ctx context.Context, c *Client, request *DescribeMultiPathGatewayRequest) (response *DescribeMultiPathGatewayResponse, err error) {
    if request == nil {
        request = NewDescribeMultiPathGatewayRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeMultiPathGateway")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeMultiPathGateway require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeMultiPathGatewayResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeMultiPathGatewayLineRequest() (request *DescribeMultiPathGatewayLineRequest) {
    request = &DescribeMultiPathGatewayLineRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeMultiPathGatewayLine")
    
    
    return
}

func NewDescribeMultiPathGatewayLineResponse() (response *DescribeMultiPathGatewayLineResponse) {
    response = &DescribeMultiPathGatewayLineResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeMultiPathGatewayLine
// 通过本接口查询接入多通道安全加速网关的线路。包括直连、EdgeOne 四层代理线路、自定义线路。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DescribeMultiPathGatewayLine(c *Client, request *DescribeMultiPathGatewayLineRequest) (response *DescribeMultiPathGatewayLineResponse, err error) {
    return DescribeMultiPathGatewayLineWithContext(context.Background(), c, request)
}

// DescribeMultiPathGatewayLine
// 通过本接口查询接入多通道安全加速网关的线路。包括直连、EdgeOne 四层代理线路、自定义线路。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DescribeMultiPathGatewayLineWithContext(ctx context.Context, c *Client, request *DescribeMultiPathGatewayLineRequest) (response *DescribeMultiPathGatewayLineResponse, err error) {
    if request == nil {
        request = NewDescribeMultiPathGatewayLineRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeMultiPathGatewayLine")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeMultiPathGatewayLine require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeMultiPathGatewayLineResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeMultiPathGatewayOriginACLRequest() (request *DescribeMultiPathGatewayOriginACLRequest) {
    request = &DescribeMultiPathGatewayOriginACLRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeMultiPathGatewayOriginACL")
    
    
    return
}

func NewDescribeMultiPathGatewayOriginACLResponse() (response *DescribeMultiPathGatewayOriginACLResponse) {
    response = &DescribeMultiPathGatewayOriginACLResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeMultiPathGatewayOriginACL
// 本接口用于查询多通道安全加速网关实例与回源 IP 网段的绑定关系，以及回源 IP 网段详情。若 MultiPathGatewayNextOriginACL 字段有返回值，则需要将最新的回源 IP 网段同步到源站防火墙配置中。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeMultiPathGatewayOriginACL(c *Client, request *DescribeMultiPathGatewayOriginACLRequest) (response *DescribeMultiPathGatewayOriginACLResponse, err error) {
    return DescribeMultiPathGatewayOriginACLWithContext(context.Background(), c, request)
}

// DescribeMultiPathGatewayOriginACL
// 本接口用于查询多通道安全加速网关实例与回源 IP 网段的绑定关系，以及回源 IP 网段详情。若 MultiPathGatewayNextOriginACL 字段有返回值，则需要将最新的回源 IP 网段同步到源站防火墙配置中。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeMultiPathGatewayOriginACLWithContext(ctx context.Context, c *Client, request *DescribeMultiPathGatewayOriginACLRequest) (response *DescribeMultiPathGatewayOriginACLResponse, err error) {
    if request == nil {
        request = NewDescribeMultiPathGatewayOriginACLRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeMultiPathGatewayOriginACL")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeMultiPathGatewayOriginACL require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeMultiPathGatewayOriginACLResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeMultiPathGatewayRegionsRequest() (request *DescribeMultiPathGatewayRegionsRequest) {
    request = &DescribeMultiPathGatewayRegionsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeMultiPathGatewayRegions")
    
    
    return
}

func NewDescribeMultiPathGatewayRegionsResponse() (response *DescribeMultiPathGatewayRegionsResponse) {
    response = &DescribeMultiPathGatewayRegionsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeMultiPathGatewayRegions
// 通过本接口查询用户创建的多通道安全加速网关（云上网关）的可用地域列表。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DescribeMultiPathGatewayRegions(c *Client, request *DescribeMultiPathGatewayRegionsRequest) (response *DescribeMultiPathGatewayRegionsResponse, err error) {
    return DescribeMultiPathGatewayRegionsWithContext(context.Background(), c, request)
}

// DescribeMultiPathGatewayRegions
// 通过本接口查询用户创建的多通道安全加速网关（云上网关）的可用地域列表。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DescribeMultiPathGatewayRegionsWithContext(ctx context.Context, c *Client, request *DescribeMultiPathGatewayRegionsRequest) (response *DescribeMultiPathGatewayRegionsResponse, err error) {
    if request == nil {
        request = NewDescribeMultiPathGatewayRegionsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeMultiPathGatewayRegions")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeMultiPathGatewayRegions require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeMultiPathGatewayRegionsResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeMultiPathGatewaySecretKeyRequest() (request *DescribeMultiPathGatewaySecretKeyRequest) {
    request = &DescribeMultiPathGatewaySecretKeyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeMultiPathGatewaySecretKey")
    
    
    return
}

func NewDescribeMultiPathGatewaySecretKeyResponse() (response *DescribeMultiPathGatewaySecretKeyResponse) {
    response = &DescribeMultiPathGatewaySecretKeyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeMultiPathGatewaySecretKey
// 通过本接口查询接入多通道安全加速网关的密钥，客户基于接入密钥签名接入多通道安全加速网关。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DescribeMultiPathGatewaySecretKey(c *Client, request *DescribeMultiPathGatewaySecretKeyRequest) (response *DescribeMultiPathGatewaySecretKeyResponse, err error) {
    return DescribeMultiPathGatewaySecretKeyWithContext(context.Background(), c, request)
}

// DescribeMultiPathGatewaySecretKey
// 通过本接口查询接入多通道安全加速网关的密钥，客户基于接入密钥签名接入多通道安全加速网关。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DescribeMultiPathGatewaySecretKeyWithContext(ctx context.Context, c *Client, request *DescribeMultiPathGatewaySecretKeyRequest) (response *DescribeMultiPathGatewaySecretKeyResponse, err error) {
    if request == nil {
        request = NewDescribeMultiPathGatewaySecretKeyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeMultiPathGatewaySecretKey")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeMultiPathGatewaySecretKey require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeMultiPathGatewaySecretKeyResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeMultiPathGatewaysRequest() (request *DescribeMultiPathGatewaysRequest) {
    request = &DescribeMultiPathGatewaysRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeMultiPathGateways")
    
    
    return
}

func NewDescribeMultiPathGatewaysResponse() (response *DescribeMultiPathGatewaysResponse) {
    response = &DescribeMultiPathGatewaysResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeMultiPathGateways
// 通过本接口查询用户创建的多通道安全加速网关列表。支持翻页。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DescribeMultiPathGateways(c *Client, request *DescribeMultiPathGatewaysRequest) (response *DescribeMultiPathGatewaysResponse, err error) {
    return DescribeMultiPathGatewaysWithContext(context.Background(), c, request)
}

// DescribeMultiPathGateways
// 通过本接口查询用户创建的多通道安全加速网关列表。支持翻页。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func DescribeMultiPathGatewaysWithContext(ctx context.Context, c *Client, request *DescribeMultiPathGatewaysRequest) (response *DescribeMultiPathGatewaysResponse, err error) {
    if request == nil {
        request = NewDescribeMultiPathGatewaysRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeMultiPathGateways")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeMultiPathGateways require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeMultiPathGatewaysResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeOriginACLRequest() (request *DescribeOriginACLRequest) {
    request = &DescribeOriginACLRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeOriginACL")
    
    
    return
}

func NewDescribeOriginACLResponse() (response *DescribeOriginACLResponse) {
    response = &DescribeOriginACLResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeOriginACL
// 本接口用于查询站点下的七层加速域名/四层代理实例与回源 IP 网段的绑定关系，以及回源 IP 网段详情。如果您想通过自动化脚本定期获取回源 IP 网段的最新版本，可以较低频率（建议每三天一次）轮询本接口，若 NextOriginACL 字段有返回值，则将最新的回源 IP 网段同步到源站防火墙配置中。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeOriginACL(c *Client, request *DescribeOriginACLRequest) (response *DescribeOriginACLResponse, err error) {
    return DescribeOriginACLWithContext(context.Background(), c, request)
}

// DescribeOriginACL
// 本接口用于查询站点下的七层加速域名/四层代理实例与回源 IP 网段的绑定关系，以及回源 IP 网段详情。如果您想通过自动化脚本定期获取回源 IP 网段的最新版本，可以较低频率（建议每三天一次）轮询本接口，若 NextOriginACL 字段有返回值，则将最新的回源 IP 网段同步到源站防火墙配置中。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeOriginACLWithContext(ctx context.Context, c *Client, request *DescribeOriginACLRequest) (response *DescribeOriginACLResponse, err error) {
    if request == nil {
        request = NewDescribeOriginACLRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeOriginACL")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeOriginACL require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeOriginACLResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeOriginGroupRequest() (request *DescribeOriginGroupRequest) {
    request = &DescribeOriginGroupRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeOriginGroup")
    
    
    return
}

func NewDescribeOriginGroupResponse() (response *DescribeOriginGroupResponse) {
    response = &DescribeOriginGroupResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeOriginGroup
// 获取源站组列表
//
// 可能返回的错误码:
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeOriginGroup(c *Client, request *DescribeOriginGroupRequest) (response *DescribeOriginGroupResponse, err error) {
    return DescribeOriginGroupWithContext(context.Background(), c, request)
}

// DescribeOriginGroup
// 获取源站组列表
//
// 可能返回的错误码:
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeOriginGroupWithContext(ctx context.Context, c *Client, request *DescribeOriginGroupRequest) (response *DescribeOriginGroupResponse, err error) {
    if request == nil {
        request = NewDescribeOriginGroupRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeOriginGroup")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeOriginGroup require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeOriginGroupResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeOriginGroupHealthStatusRequest() (request *DescribeOriginGroupHealthStatusRequest) {
    request = &DescribeOriginGroupHealthStatusRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeOriginGroupHealthStatus")
    
    
    return
}

func NewDescribeOriginGroupHealthStatusResponse() (response *DescribeOriginGroupHealthStatusResponse) {
    response = &DescribeOriginGroupHealthStatusResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeOriginGroupHealthStatus
// 查询负载均衡实例下源站组健康状态。负载均衡功能内测中，如您需要使用请 [联系我们](https://cloud.tencent.com/online-service)。
//
// 可能返回的错误码:
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeOriginGroupHealthStatus(c *Client, request *DescribeOriginGroupHealthStatusRequest) (response *DescribeOriginGroupHealthStatusResponse, err error) {
    return DescribeOriginGroupHealthStatusWithContext(context.Background(), c, request)
}

// DescribeOriginGroupHealthStatus
// 查询负载均衡实例下源站组健康状态。负载均衡功能内测中，如您需要使用请 [联系我们](https://cloud.tencent.com/online-service)。
//
// 可能返回的错误码:
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeOriginGroupHealthStatusWithContext(ctx context.Context, c *Client, request *DescribeOriginGroupHealthStatusRequest) (response *DescribeOriginGroupHealthStatusResponse, err error) {
    if request == nil {
        request = NewDescribeOriginGroupHealthStatusRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeOriginGroupHealthStatus")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeOriginGroupHealthStatus require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeOriginGroupHealthStatusResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeOriginProtectionRequest() (request *DescribeOriginProtectionRequest) {
    request = &DescribeOriginProtectionRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeOriginProtection")
    
    
    return
}

func NewDescribeOriginProtectionResponse() (response *DescribeOriginProtectionResponse) {
    response = &DescribeOriginProtectionResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeOriginProtection
// 本接口为旧版本查询源站防护接口，EdgeOne 于 2025 年 6 月 27 日已对源站防护相关接口全面升级，新版本查询源站防护接口详情请参考 [DescribeOriginACL](https://cloud.tencent.com/document/product/1552/120408)。
//
// 
//
// <p style="color: red;">注意：自 2025 年 6 月 27 日起，旧版接口停止更新迭代，后续新增功能将仅在新版接口中提供。为避免在使用旧版接口时出现数据字段冲突，建议您尽早迁移到新版源站防护接口。</p>
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeOriginProtection(c *Client, request *DescribeOriginProtectionRequest) (response *DescribeOriginProtectionResponse, err error) {
    return DescribeOriginProtectionWithContext(context.Background(), c, request)
}

// DescribeOriginProtection
// 本接口为旧版本查询源站防护接口，EdgeOne 于 2025 年 6 月 27 日已对源站防护相关接口全面升级，新版本查询源站防护接口详情请参考 [DescribeOriginACL](https://cloud.tencent.com/document/product/1552/120408)。
//
// 
//
// <p style="color: red;">注意：自 2025 年 6 月 27 日起，旧版接口停止更新迭代，后续新增功能将仅在新版接口中提供。为避免在使用旧版接口时出现数据字段冲突，建议您尽早迁移到新版源站防护接口。</p>
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeOriginProtectionWithContext(ctx context.Context, c *Client, request *DescribeOriginProtectionRequest) (response *DescribeOriginProtectionResponse, err error) {
    if request == nil {
        request = NewDescribeOriginProtectionRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeOriginProtection")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeOriginProtection require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeOriginProtectionResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeOverviewL7DataRequest() (request *DescribeOverviewL7DataRequest) {
    request = &DescribeOverviewL7DataRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeOverviewL7Data")
    
    
    return
}

func NewDescribeOverviewL7DataResponse() (response *DescribeOverviewL7DataResponse) {
    response = &DescribeOverviewL7DataResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeOverviewL7Data
// 本接口（DescribeOverviewL7Data）用于查询七层监控类时序流量数据。此接口待废弃，请使用 <a href="https://cloud.tencent.com/document/product/1552/80648">DescribeTimingL7AnalysisData</a> 接口。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeOverviewL7Data(c *Client, request *DescribeOverviewL7DataRequest) (response *DescribeOverviewL7DataResponse, err error) {
    return DescribeOverviewL7DataWithContext(context.Background(), c, request)
}

// DescribeOverviewL7Data
// 本接口（DescribeOverviewL7Data）用于查询七层监控类时序流量数据。此接口待废弃，请使用 <a href="https://cloud.tencent.com/document/product/1552/80648">DescribeTimingL7AnalysisData</a> 接口。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeOverviewL7DataWithContext(ctx context.Context, c *Client, request *DescribeOverviewL7DataRequest) (response *DescribeOverviewL7DataResponse, err error) {
    if request == nil {
        request = NewDescribeOverviewL7DataRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeOverviewL7Data")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeOverviewL7Data require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeOverviewL7DataResponse()
    err = c.Send(request, response)
    return
}

func NewDescribePlansRequest() (request *DescribePlansRequest) {
    request = &DescribePlansRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribePlans")
    
    
    return
}

func NewDescribePlansResponse() (response *DescribePlansResponse) {
    response = &DescribePlansResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribePlans
// 查询套餐信息列表，支持分页。
//
// 可能返回的错误码:
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func DescribePlans(c *Client, request *DescribePlansRequest) (response *DescribePlansResponse, err error) {
    return DescribePlansWithContext(context.Background(), c, request)
}

// DescribePlans
// 查询套餐信息列表，支持分页。
//
// 可能返回的错误码:
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func DescribePlansWithContext(ctx context.Context, c *Client, request *DescribePlansRequest) (response *DescribePlansResponse, err error) {
    if request == nil {
        request = NewDescribePlansRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribePlans")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribePlans require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribePlansResponse()
    err = c.Send(request, response)
    return
}

func NewDescribePrefetchTasksRequest() (request *DescribePrefetchTasksRequest) {
    request = &DescribePrefetchTasksRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribePrefetchTasks")
    
    
    return
}

func NewDescribePrefetchTasksResponse() (response *DescribePrefetchTasksResponse) {
    response = &DescribePrefetchTasksResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribePrefetchTasks
// DescribePrefetchTasks 用于查询预热任务提交历史记录及执行进度，通过 CreatePrefetchTasks 接口提交的任务可通过此接口进行查询。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_DOMAINEMPTY = "UnauthorizedOperation.DomainEmpty"
func DescribePrefetchTasks(c *Client, request *DescribePrefetchTasksRequest) (response *DescribePrefetchTasksResponse, err error) {
    return DescribePrefetchTasksWithContext(context.Background(), c, request)
}

// DescribePrefetchTasks
// DescribePrefetchTasks 用于查询预热任务提交历史记录及执行进度，通过 CreatePrefetchTasks 接口提交的任务可通过此接口进行查询。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_DOMAINEMPTY = "UnauthorizedOperation.DomainEmpty"
func DescribePrefetchTasksWithContext(ctx context.Context, c *Client, request *DescribePrefetchTasksRequest) (response *DescribePrefetchTasksResponse, err error) {
    if request == nil {
        request = NewDescribePrefetchTasksRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribePrefetchTasks")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribePrefetchTasks require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribePrefetchTasksResponse()
    err = c.Send(request, response)
    return
}

func NewDescribePurgeTasksRequest() (request *DescribePurgeTasksRequest) {
    request = &DescribePurgeTasksRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribePurgeTasks")
    
    
    return
}

func NewDescribePurgeTasksResponse() (response *DescribePurgeTasksResponse) {
    response = &DescribePurgeTasksResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribePurgeTasks
// DescribePurgeTasks 用于查询提交的 URL 刷新、目录刷新记录及执行进度，通过 CreatePurgeTasks 接口提交的任务均可通过此接口进行查询。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  OPERATIONDENIED = "OperationDenied"
func DescribePurgeTasks(c *Client, request *DescribePurgeTasksRequest) (response *DescribePurgeTasksResponse, err error) {
    return DescribePurgeTasksWithContext(context.Background(), c, request)
}

// DescribePurgeTasks
// DescribePurgeTasks 用于查询提交的 URL 刷新、目录刷新记录及执行进度，通过 CreatePurgeTasks 接口提交的任务均可通过此接口进行查询。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  OPERATIONDENIED = "OperationDenied"
func DescribePurgeTasksWithContext(ctx context.Context, c *Client, request *DescribePurgeTasksRequest) (response *DescribePurgeTasksResponse, err error) {
    if request == nil {
        request = NewDescribePurgeTasksRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribePurgeTasks")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribePurgeTasks require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribePurgeTasksResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeRealtimeLogDeliveryTasksRequest() (request *DescribeRealtimeLogDeliveryTasksRequest) {
    request = &DescribeRealtimeLogDeliveryTasksRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeRealtimeLogDeliveryTasks")
    
    
    return
}

func NewDescribeRealtimeLogDeliveryTasksResponse() (response *DescribeRealtimeLogDeliveryTasksResponse) {
    response = &DescribeRealtimeLogDeliveryTasksResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeRealtimeLogDeliveryTasks
// 通过本接口查询实时日志投递任务列表。
//
// 可能返回的错误码:
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeRealtimeLogDeliveryTasks(c *Client, request *DescribeRealtimeLogDeliveryTasksRequest) (response *DescribeRealtimeLogDeliveryTasksResponse, err error) {
    return DescribeRealtimeLogDeliveryTasksWithContext(context.Background(), c, request)
}

// DescribeRealtimeLogDeliveryTasks
// 通过本接口查询实时日志投递任务列表。
//
// 可能返回的错误码:
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeRealtimeLogDeliveryTasksWithContext(ctx context.Context, c *Client, request *DescribeRealtimeLogDeliveryTasksRequest) (response *DescribeRealtimeLogDeliveryTasksResponse, err error) {
    if request == nil {
        request = NewDescribeRealtimeLogDeliveryTasksRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeRealtimeLogDeliveryTasks")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeRealtimeLogDeliveryTasks require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeRealtimeLogDeliveryTasksResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeRulesRequest() (request *DescribeRulesRequest) {
    request = &DescribeRulesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeRules")
    
    
    return
}

func NewDescribeRulesResponse() (response *DescribeRulesResponse) {
    response = &DescribeRulesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeRules
// 本接口为旧版本查询规则引擎规则接口，EdgeOne 于 2025 年 1 月 21 日已对规则引擎相关接口全面升级，新版本查询七层加速规则接口详情请参考  [DescribeL7AccRules](https://cloud.tencent.com/document/product/1552/115820)。
//
// <p style="color: red;">注意：自 2025 年 1 月 21 日起，旧版接口停止更新迭代，后续新增功能将仅在新版接口中提供，旧版接口支持的原有能力将不受影响。为避免在使用旧版接口时出现数据字段冲突，建议您尽早迁移到新版规则引擎接口。</p>
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeRules(c *Client, request *DescribeRulesRequest) (response *DescribeRulesResponse, err error) {
    return DescribeRulesWithContext(context.Background(), c, request)
}

// DescribeRules
// 本接口为旧版本查询规则引擎规则接口，EdgeOne 于 2025 年 1 月 21 日已对规则引擎相关接口全面升级，新版本查询七层加速规则接口详情请参考  [DescribeL7AccRules](https://cloud.tencent.com/document/product/1552/115820)。
//
// <p style="color: red;">注意：自 2025 年 1 月 21 日起，旧版接口停止更新迭代，后续新增功能将仅在新版接口中提供，旧版接口支持的原有能力将不受影响。为避免在使用旧版接口时出现数据字段冲突，建议您尽早迁移到新版规则引擎接口。</p>
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeRulesWithContext(ctx context.Context, c *Client, request *DescribeRulesRequest) (response *DescribeRulesResponse, err error) {
    if request == nil {
        request = NewDescribeRulesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeRules")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeRules require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeRulesResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeRulesSettingRequest() (request *DescribeRulesSettingRequest) {
    request = &DescribeRulesSettingRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeRulesSetting")
    
    
    return
}

func NewDescribeRulesSettingResponse() (response *DescribeRulesSettingResponse) {
    response = &DescribeRulesSettingResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeRulesSetting
// 本接口为旧版，EdgeOne 已对规则引擎相关接口全面升级，详情请参考 [RuleEngineAction](https://cloud.tencent.com/document/product/1552/80721#RuleEngineAction)。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeRulesSetting(c *Client, request *DescribeRulesSettingRequest) (response *DescribeRulesSettingResponse, err error) {
    return DescribeRulesSettingWithContext(context.Background(), c, request)
}

// DescribeRulesSetting
// 本接口为旧版，EdgeOne 已对规则引擎相关接口全面升级，详情请参考 [RuleEngineAction](https://cloud.tencent.com/document/product/1552/80721#RuleEngineAction)。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeRulesSettingWithContext(ctx context.Context, c *Client, request *DescribeRulesSettingRequest) (response *DescribeRulesSettingResponse, err error) {
    if request == nil {
        request = NewDescribeRulesSettingRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeRulesSetting")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeRulesSetting require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeRulesSettingResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeSecurityAPIResourceRequest() (request *DescribeSecurityAPIResourceRequest) {
    request = &DescribeSecurityAPIResourceRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeSecurityAPIResource")
    
    
    return
}

func NewDescribeSecurityAPIResourceResponse() (response *DescribeSecurityAPIResourceResponse) {
    response = &DescribeSecurityAPIResourceResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeSecurityAPIResource
// 查询站点下的 API 资源。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityAPIResource(c *Client, request *DescribeSecurityAPIResourceRequest) (response *DescribeSecurityAPIResourceResponse, err error) {
    return DescribeSecurityAPIResourceWithContext(context.Background(), c, request)
}

// DescribeSecurityAPIResource
// 查询站点下的 API 资源。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityAPIResourceWithContext(ctx context.Context, c *Client, request *DescribeSecurityAPIResourceRequest) (response *DescribeSecurityAPIResourceResponse, err error) {
    if request == nil {
        request = NewDescribeSecurityAPIResourceRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeSecurityAPIResource")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeSecurityAPIResource require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeSecurityAPIResourceResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeSecurityAPIServiceRequest() (request *DescribeSecurityAPIServiceRequest) {
    request = &DescribeSecurityAPIServiceRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeSecurityAPIService")
    
    
    return
}

func NewDescribeSecurityAPIServiceResponse() (response *DescribeSecurityAPIServiceResponse) {
    response = &DescribeSecurityAPIServiceResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeSecurityAPIService
// 查询站点下的 API 服务。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityAPIService(c *Client, request *DescribeSecurityAPIServiceRequest) (response *DescribeSecurityAPIServiceResponse, err error) {
    return DescribeSecurityAPIServiceWithContext(context.Background(), c, request)
}

// DescribeSecurityAPIService
// 查询站点下的 API 服务。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityAPIServiceWithContext(ctx context.Context, c *Client, request *DescribeSecurityAPIServiceRequest) (response *DescribeSecurityAPIServiceResponse, err error) {
    if request == nil {
        request = NewDescribeSecurityAPIServiceRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeSecurityAPIService")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeSecurityAPIService require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeSecurityAPIServiceResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeSecurityClientAttesterRequest() (request *DescribeSecurityClientAttesterRequest) {
    request = &DescribeSecurityClientAttesterRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeSecurityClientAttester")
    
    
    return
}

func NewDescribeSecurityClientAttesterResponse() (response *DescribeSecurityClientAttesterResponse) {
    response = &DescribeSecurityClientAttesterResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeSecurityClientAttester
// 查询客户端认证选项配置。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityClientAttester(c *Client, request *DescribeSecurityClientAttesterRequest) (response *DescribeSecurityClientAttesterResponse, err error) {
    return DescribeSecurityClientAttesterWithContext(context.Background(), c, request)
}

// DescribeSecurityClientAttester
// 查询客户端认证选项配置。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityClientAttesterWithContext(ctx context.Context, c *Client, request *DescribeSecurityClientAttesterRequest) (response *DescribeSecurityClientAttesterResponse, err error) {
    if request == nil {
        request = NewDescribeSecurityClientAttesterRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeSecurityClientAttester")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeSecurityClientAttester require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeSecurityClientAttesterResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeSecurityIPGroupRequest() (request *DescribeSecurityIPGroupRequest) {
    request = &DescribeSecurityIPGroupRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeSecurityIPGroup")
    
    
    return
}

func NewDescribeSecurityIPGroupResponse() (response *DescribeSecurityIPGroupResponse) {
    response = &DescribeSecurityIPGroupResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeSecurityIPGroup
// 查询安全 IP 组的配置信息，包括安全 IP 组的 ID、名称和内容。本接口的查询结果中，每个 IP 组最多只返回 2000 个 IP / 网段。如果存在超过 2000 个 IP / 网段的超大 IP 组，请调用 DescribeSecurityIPGroupContent 进行分页查询。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityIPGroup(c *Client, request *DescribeSecurityIPGroupRequest) (response *DescribeSecurityIPGroupResponse, err error) {
    return DescribeSecurityIPGroupWithContext(context.Background(), c, request)
}

// DescribeSecurityIPGroup
// 查询安全 IP 组的配置信息，包括安全 IP 组的 ID、名称和内容。本接口的查询结果中，每个 IP 组最多只返回 2000 个 IP / 网段。如果存在超过 2000 个 IP / 网段的超大 IP 组，请调用 DescribeSecurityIPGroupContent 进行分页查询。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityIPGroupWithContext(ctx context.Context, c *Client, request *DescribeSecurityIPGroupRequest) (response *DescribeSecurityIPGroupResponse, err error) {
    if request == nil {
        request = NewDescribeSecurityIPGroupRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeSecurityIPGroup")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeSecurityIPGroup require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeSecurityIPGroupResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeSecurityIPGroupContentRequest() (request *DescribeSecurityIPGroupContentRequest) {
    request = &DescribeSecurityIPGroupContentRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeSecurityIPGroupContent")
    
    
    return
}

func NewDescribeSecurityIPGroupContentResponse() (response *DescribeSecurityIPGroupContentResponse) {
    response = &DescribeSecurityIPGroupContentResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeSecurityIPGroupContent
// 该接口用于分页查询指定 IP 组中的 IP 地址列表。当 IP 组中的 IP 地址数量超过 2000 个时，可以使用此接口进行分页查询，以获取完整的 IP 地址列表。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityIPGroupContent(c *Client, request *DescribeSecurityIPGroupContentRequest) (response *DescribeSecurityIPGroupContentResponse, err error) {
    return DescribeSecurityIPGroupContentWithContext(context.Background(), c, request)
}

// DescribeSecurityIPGroupContent
// 该接口用于分页查询指定 IP 组中的 IP 地址列表。当 IP 组中的 IP 地址数量超过 2000 个时，可以使用此接口进行分页查询，以获取完整的 IP 地址列表。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityIPGroupContentWithContext(ctx context.Context, c *Client, request *DescribeSecurityIPGroupContentRequest) (response *DescribeSecurityIPGroupContentResponse, err error) {
    if request == nil {
        request = NewDescribeSecurityIPGroupContentRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeSecurityIPGroupContent")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeSecurityIPGroupContent require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeSecurityIPGroupContentResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeSecurityIPGroupInfoRequest() (request *DescribeSecurityIPGroupInfoRequest) {
    request = &DescribeSecurityIPGroupInfoRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeSecurityIPGroupInfo")
    
    
    return
}

func NewDescribeSecurityIPGroupInfoResponse() (response *DescribeSecurityIPGroupInfoResponse) {
    response = &DescribeSecurityIPGroupInfoResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeSecurityIPGroupInfo
// 接口已废弃，将于 2024 年 6 月 30 日停止服务。请使用 [查询安全 IP 组
//
// ](https://cloud.tencent.com/document/product/1552/105866) 接口。
//
// 
//
// 查询 IP 组的配置信息，包括 IP 组名称、 IP 组内容、 IP 组归属站点。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityIPGroupInfo(c *Client, request *DescribeSecurityIPGroupInfoRequest) (response *DescribeSecurityIPGroupInfoResponse, err error) {
    return DescribeSecurityIPGroupInfoWithContext(context.Background(), c, request)
}

// DescribeSecurityIPGroupInfo
// 接口已废弃，将于 2024 年 6 月 30 日停止服务。请使用 [查询安全 IP 组
//
// ](https://cloud.tencent.com/document/product/1552/105866) 接口。
//
// 
//
// 查询 IP 组的配置信息，包括 IP 组名称、 IP 组内容、 IP 组归属站点。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityIPGroupInfoWithContext(ctx context.Context, c *Client, request *DescribeSecurityIPGroupInfoRequest) (response *DescribeSecurityIPGroupInfoResponse, err error) {
    if request == nil {
        request = NewDescribeSecurityIPGroupInfoRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeSecurityIPGroupInfo")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeSecurityIPGroupInfo require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeSecurityIPGroupInfoResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeSecurityJSInjectionRuleRequest() (request *DescribeSecurityJSInjectionRuleRequest) {
    request = &DescribeSecurityJSInjectionRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeSecurityJSInjectionRule")
    
    
    return
}

func NewDescribeSecurityJSInjectionRuleResponse() (response *DescribeSecurityJSInjectionRuleResponse) {
    response = &DescribeSecurityJSInjectionRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeSecurityJSInjectionRule
// 查询 JavaScript 注入规则。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityJSInjectionRule(c *Client, request *DescribeSecurityJSInjectionRuleRequest) (response *DescribeSecurityJSInjectionRuleResponse, err error) {
    return DescribeSecurityJSInjectionRuleWithContext(context.Background(), c, request)
}

// DescribeSecurityJSInjectionRule
// 查询 JavaScript 注入规则。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityJSInjectionRuleWithContext(ctx context.Context, c *Client, request *DescribeSecurityJSInjectionRuleRequest) (response *DescribeSecurityJSInjectionRuleResponse, err error) {
    if request == nil {
        request = NewDescribeSecurityJSInjectionRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeSecurityJSInjectionRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeSecurityJSInjectionRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeSecurityJSInjectionRuleResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeSecurityPolicyRequest() (request *DescribeSecurityPolicyRequest) {
    request = &DescribeSecurityPolicyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeSecurityPolicy")
    
    
    return
}

func NewDescribeSecurityPolicyResponse() (response *DescribeSecurityPolicyResponse) {
    response = &DescribeSecurityPolicyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeSecurityPolicy
// 查询安全防护配置详情。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityPolicy(c *Client, request *DescribeSecurityPolicyRequest) (response *DescribeSecurityPolicyResponse, err error) {
    return DescribeSecurityPolicyWithContext(context.Background(), c, request)
}

// DescribeSecurityPolicy
// 查询安全防护配置详情。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityPolicyWithContext(ctx context.Context, c *Client, request *DescribeSecurityPolicyRequest) (response *DescribeSecurityPolicyResponse, err error) {
    if request == nil {
        request = NewDescribeSecurityPolicyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeSecurityPolicy")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeSecurityPolicy require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeSecurityPolicyResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeSecurityTemplateBindingsRequest() (request *DescribeSecurityTemplateBindingsRequest) {
    request = &DescribeSecurityTemplateBindingsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeSecurityTemplateBindings")
    
    
    return
}

func NewDescribeSecurityTemplateBindingsResponse() (response *DescribeSecurityTemplateBindingsResponse) {
    response = &DescribeSecurityTemplateBindingsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeSecurityTemplateBindings
// 查询指定策略模板的绑定关系列表。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityTemplateBindings(c *Client, request *DescribeSecurityTemplateBindingsRequest) (response *DescribeSecurityTemplateBindingsResponse, err error) {
    return DescribeSecurityTemplateBindingsWithContext(context.Background(), c, request)
}

// DescribeSecurityTemplateBindings
// 查询指定策略模板的绑定关系列表。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeSecurityTemplateBindingsWithContext(ctx context.Context, c *Client, request *DescribeSecurityTemplateBindingsRequest) (response *DescribeSecurityTemplateBindingsResponse, err error) {
    if request == nil {
        request = NewDescribeSecurityTemplateBindingsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeSecurityTemplateBindings")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeSecurityTemplateBindings require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeSecurityTemplateBindingsResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeTimingL4DataRequest() (request *DescribeTimingL4DataRequest) {
    request = &DescribeTimingL4DataRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeTimingL4Data")
    
    
    return
}

func NewDescribeTimingL4DataResponse() (response *DescribeTimingL4DataResponse) {
    response = &DescribeTimingL4DataResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeTimingL4Data
// 本接口（DescribeTimingL4Data）用于查询四层时序流量数据列表。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  LIMITEXCEEDED = "LimitExceeded"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeTimingL4Data(c *Client, request *DescribeTimingL4DataRequest) (response *DescribeTimingL4DataResponse, err error) {
    return DescribeTimingL4DataWithContext(context.Background(), c, request)
}

// DescribeTimingL4Data
// 本接口（DescribeTimingL4Data）用于查询四层时序流量数据列表。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  LIMITEXCEEDED = "LimitExceeded"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeTimingL4DataWithContext(ctx context.Context, c *Client, request *DescribeTimingL4DataRequest) (response *DescribeTimingL4DataResponse, err error) {
    if request == nil {
        request = NewDescribeTimingL4DataRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeTimingL4Data")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeTimingL4Data require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeTimingL4DataResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeTimingL7AnalysisDataRequest() (request *DescribeTimingL7AnalysisDataRequest) {
    request = &DescribeTimingL7AnalysisDataRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeTimingL7AnalysisData")
    
    
    return
}

func NewDescribeTimingL7AnalysisDataResponse() (response *DescribeTimingL7AnalysisDataResponse) {
    response = &DescribeTimingL7AnalysisDataResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeTimingL7AnalysisData
// 本接口用于查询七层域名业务的时序数据。
//
// 注意：
//
// 1. 本接口查询数据有 10 分钟左右延迟，建议拉取当前时间 10 分钟以前的数据。
//
// 2. 本接口默认返回防护后的流量请求数据，用户可在 `Filters.mitigatedByWebSecurity` 中自定义查询已防护缓释的数据。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeTimingL7AnalysisData(c *Client, request *DescribeTimingL7AnalysisDataRequest) (response *DescribeTimingL7AnalysisDataResponse, err error) {
    return DescribeTimingL7AnalysisDataWithContext(context.Background(), c, request)
}

// DescribeTimingL7AnalysisData
// 本接口用于查询七层域名业务的时序数据。
//
// 注意：
//
// 1. 本接口查询数据有 10 分钟左右延迟，建议拉取当前时间 10 分钟以前的数据。
//
// 2. 本接口默认返回防护后的流量请求数据，用户可在 `Filters.mitigatedByWebSecurity` 中自定义查询已防护缓释的数据。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeTimingL7AnalysisDataWithContext(ctx context.Context, c *Client, request *DescribeTimingL7AnalysisDataRequest) (response *DescribeTimingL7AnalysisDataResponse, err error) {
    if request == nil {
        request = NewDescribeTimingL7AnalysisDataRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeTimingL7AnalysisData")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeTimingL7AnalysisData require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeTimingL7AnalysisDataResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeTimingL7CacheDataRequest() (request *DescribeTimingL7CacheDataRequest) {
    request = &DescribeTimingL7CacheDataRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeTimingL7CacheData")
    
    
    return
}

func NewDescribeTimingL7CacheDataResponse() (response *DescribeTimingL7CacheDataResponse) {
    response = &DescribeTimingL7CacheDataResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeTimingL7CacheData
// 本接口用于查询七层缓存分析时序类流量数据。此接口待废弃，请使用 <a href="https://cloud.tencent.com/document/product/1552/80648">DescribeTimingL7AnalysisData</a> 接口。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  LIMITEXCEEDED = "LimitExceeded"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeTimingL7CacheData(c *Client, request *DescribeTimingL7CacheDataRequest) (response *DescribeTimingL7CacheDataResponse, err error) {
    return DescribeTimingL7CacheDataWithContext(context.Background(), c, request)
}

// DescribeTimingL7CacheData
// 本接口用于查询七层缓存分析时序类流量数据。此接口待废弃，请使用 <a href="https://cloud.tencent.com/document/product/1552/80648">DescribeTimingL7AnalysisData</a> 接口。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  LIMITEXCEEDED = "LimitExceeded"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeTimingL7CacheDataWithContext(ctx context.Context, c *Client, request *DescribeTimingL7CacheDataRequest) (response *DescribeTimingL7CacheDataResponse, err error) {
    if request == nil {
        request = NewDescribeTimingL7CacheDataRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeTimingL7CacheData")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeTimingL7CacheData require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeTimingL7CacheDataResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeTimingL7OriginPullDataRequest() (request *DescribeTimingL7OriginPullDataRequest) {
    request = &DescribeTimingL7OriginPullDataRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeTimingL7OriginPullData")
    
    
    return
}

func NewDescribeTimingL7OriginPullDataResponse() (response *DescribeTimingL7OriginPullDataResponse) {
    response = &DescribeTimingL7OriginPullDataResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeTimingL7OriginPullData
// 本接口用以查询七层域名业务的回源时序数据。
//
// 可能返回的错误码:
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  OPERATIONDENIED_ORIGINPULLDATANOTSUPPORTED = "OperationDenied.OriginPullDataNotSupported"
func DescribeTimingL7OriginPullData(c *Client, request *DescribeTimingL7OriginPullDataRequest) (response *DescribeTimingL7OriginPullDataResponse, err error) {
    return DescribeTimingL7OriginPullDataWithContext(context.Background(), c, request)
}

// DescribeTimingL7OriginPullData
// 本接口用以查询七层域名业务的回源时序数据。
//
// 可能返回的错误码:
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  OPERATIONDENIED_ORIGINPULLDATANOTSUPPORTED = "OperationDenied.OriginPullDataNotSupported"
func DescribeTimingL7OriginPullDataWithContext(ctx context.Context, c *Client, request *DescribeTimingL7OriginPullDataRequest) (response *DescribeTimingL7OriginPullDataResponse, err error) {
    if request == nil {
        request = NewDescribeTimingL7OriginPullDataRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeTimingL7OriginPullData")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeTimingL7OriginPullData require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeTimingL7OriginPullDataResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeTopL7AnalysisDataRequest() (request *DescribeTopL7AnalysisDataRequest) {
    request = &DescribeTopL7AnalysisDataRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeTopL7AnalysisData")
    
    
    return
}

func NewDescribeTopL7AnalysisDataResponse() (response *DescribeTopL7AnalysisDataResponse) {
    response = &DescribeTopL7AnalysisDataResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeTopL7AnalysisData
// 本接口用于查询七层域名业务按照指定维度的 topN 数据。
//
// 注意：
//
// 1. 本接口查询数据有 10 分钟左右延迟，建议拉取当前时间 10 分钟以前的数据。
//
// 2. 本接口默认返回防护后的流量请求数据，用户可在 `Filters.mitigatedByWebSecurity` 中自定义查询已防护缓释的数据。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func DescribeTopL7AnalysisData(c *Client, request *DescribeTopL7AnalysisDataRequest) (response *DescribeTopL7AnalysisDataResponse, err error) {
    return DescribeTopL7AnalysisDataWithContext(context.Background(), c, request)
}

// DescribeTopL7AnalysisData
// 本接口用于查询七层域名业务按照指定维度的 topN 数据。
//
// 注意：
//
// 1. 本接口查询数据有 10 分钟左右延迟，建议拉取当前时间 10 分钟以前的数据。
//
// 2. 本接口默认返回防护后的流量请求数据，用户可在 `Filters.mitigatedByWebSecurity` 中自定义查询已防护缓释的数据。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func DescribeTopL7AnalysisDataWithContext(ctx context.Context, c *Client, request *DescribeTopL7AnalysisDataRequest) (response *DescribeTopL7AnalysisDataResponse, err error) {
    if request == nil {
        request = NewDescribeTopL7AnalysisDataRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeTopL7AnalysisData")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeTopL7AnalysisData require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeTopL7AnalysisDataResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeTopL7CacheDataRequest() (request *DescribeTopL7CacheDataRequest) {
    request = &DescribeTopL7CacheDataRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeTopL7CacheData")
    
    
    return
}

func NewDescribeTopL7CacheDataResponse() (response *DescribeTopL7CacheDataResponse) {
    response = &DescribeTopL7CacheDataResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeTopL7CacheData
// 本接口用于查询七层缓存分析 topN 数据。此接口待废弃，请使用 <a href="https://cloud.tencent.com/document/product/1552/80646"> DescribeTopL7AnalysisData</a> 接口。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeTopL7CacheData(c *Client, request *DescribeTopL7CacheDataRequest) (response *DescribeTopL7CacheDataResponse, err error) {
    return DescribeTopL7CacheDataWithContext(context.Background(), c, request)
}

// DescribeTopL7CacheData
// 本接口用于查询七层缓存分析 topN 数据。此接口待废弃，请使用 <a href="https://cloud.tencent.com/document/product/1552/80646"> DescribeTopL7AnalysisData</a> 接口。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeTopL7CacheDataWithContext(ctx context.Context, c *Client, request *DescribeTopL7CacheDataRequest) (response *DescribeTopL7CacheDataResponse, err error) {
    if request == nil {
        request = NewDescribeTopL7CacheDataRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeTopL7CacheData")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeTopL7CacheData require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeTopL7CacheDataResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeWebSecurityTemplateRequest() (request *DescribeWebSecurityTemplateRequest) {
    request = &DescribeWebSecurityTemplateRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeWebSecurityTemplate")
    
    
    return
}

func NewDescribeWebSecurityTemplateResponse() (response *DescribeWebSecurityTemplateResponse) {
    response = &DescribeWebSecurityTemplateResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeWebSecurityTemplate
// 查询安全策略配置模板详情
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeWebSecurityTemplate(c *Client, request *DescribeWebSecurityTemplateRequest) (response *DescribeWebSecurityTemplateResponse, err error) {
    return DescribeWebSecurityTemplateWithContext(context.Background(), c, request)
}

// DescribeWebSecurityTemplate
// 查询安全策略配置模板详情
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  LIMITEXCEEDED_QUERYTIMELIMITEXCEEDED = "LimitExceeded.QueryTimeLimitExceeded"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DescribeWebSecurityTemplateWithContext(ctx context.Context, c *Client, request *DescribeWebSecurityTemplateRequest) (response *DescribeWebSecurityTemplateResponse, err error) {
    if request == nil {
        request = NewDescribeWebSecurityTemplateRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeWebSecurityTemplate")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeWebSecurityTemplate require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeWebSecurityTemplateResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeWebSecurityTemplatesRequest() (request *DescribeWebSecurityTemplatesRequest) {
    request = &DescribeWebSecurityTemplatesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeWebSecurityTemplates")
    
    
    return
}

func NewDescribeWebSecurityTemplatesResponse() (response *DescribeWebSecurityTemplatesResponse) {
    response = &DescribeWebSecurityTemplatesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeWebSecurityTemplates
// 查询安全策略配置模板列表
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeWebSecurityTemplates(c *Client, request *DescribeWebSecurityTemplatesRequest) (response *DescribeWebSecurityTemplatesResponse, err error) {
    return DescribeWebSecurityTemplatesWithContext(context.Background(), c, request)
}

// DescribeWebSecurityTemplates
// 查询安全策略配置模板列表
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeWebSecurityTemplatesWithContext(ctx context.Context, c *Client, request *DescribeWebSecurityTemplatesRequest) (response *DescribeWebSecurityTemplatesResponse, err error) {
    if request == nil {
        request = NewDescribeWebSecurityTemplatesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeWebSecurityTemplates")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeWebSecurityTemplates require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeWebSecurityTemplatesResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeZoneConfigImportResultRequest() (request *DescribeZoneConfigImportResultRequest) {
    request = &DescribeZoneConfigImportResultRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeZoneConfigImportResult")
    
    
    return
}

func NewDescribeZoneConfigImportResultResponse() (response *DescribeZoneConfigImportResultResponse) {
    response = &DescribeZoneConfigImportResultResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeZoneConfigImportResult
// 查询站点配置项导入结果接口，本接口用于站点配置导入接口（ImportZoneConfig）的结果查询。该功能仅支持标准版或企业版套餐的站点使用。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeZoneConfigImportResult(c *Client, request *DescribeZoneConfigImportResultRequest) (response *DescribeZoneConfigImportResultResponse, err error) {
    return DescribeZoneConfigImportResultWithContext(context.Background(), c, request)
}

// DescribeZoneConfigImportResult
// 查询站点配置项导入结果接口，本接口用于站点配置导入接口（ImportZoneConfig）的结果查询。该功能仅支持标准版或企业版套餐的站点使用。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeZoneConfigImportResultWithContext(ctx context.Context, c *Client, request *DescribeZoneConfigImportResultRequest) (response *DescribeZoneConfigImportResultResponse, err error) {
    if request == nil {
        request = NewDescribeZoneConfigImportResultRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeZoneConfigImportResult")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeZoneConfigImportResult require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeZoneConfigImportResultResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeZoneSettingRequest() (request *DescribeZoneSettingRequest) {
    request = &DescribeZoneSettingRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeZoneSetting")
    
    
    return
}

func NewDescribeZoneSettingResponse() (response *DescribeZoneSettingResponse) {
    response = &DescribeZoneSettingResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeZoneSetting
// 本接口为旧版，EdgeOne 已对规则引擎相关接口全面升级，详情请参考 [DescribeL7AccSetting](https://cloud.tencent.com/document/product/1552/115819)。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_SETTINGINVALIDPARAM = "InvalidParameter.SettingInvalidParam"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeZoneSetting(c *Client, request *DescribeZoneSettingRequest) (response *DescribeZoneSettingResponse, err error) {
    return DescribeZoneSettingWithContext(context.Background(), c, request)
}

// DescribeZoneSetting
// 本接口为旧版，EdgeOne 已对规则引擎相关接口全面升级，详情请参考 [DescribeL7AccSetting](https://cloud.tencent.com/document/product/1552/115819)。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_SETTINGINVALIDPARAM = "InvalidParameter.SettingInvalidParam"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeZoneSettingWithContext(ctx context.Context, c *Client, request *DescribeZoneSettingRequest) (response *DescribeZoneSettingResponse, err error) {
    if request == nil {
        request = NewDescribeZoneSettingRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeZoneSetting")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeZoneSetting require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeZoneSettingResponse()
    err = c.Send(request, response)
    return
}

func NewDescribeZonesRequest() (request *DescribeZonesRequest) {
    request = &DescribeZonesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DescribeZones")
    
    
    return
}

func NewDescribeZonesResponse() (response *DescribeZonesResponse) {
    response = &DescribeZonesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DescribeZones
// 该接口用于查询您有权限的站点信息。可根据不同查询条件筛选站点。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeZones(c *Client, request *DescribeZonesRequest) (response *DescribeZonesResponse, err error) {
    return DescribeZonesWithContext(context.Background(), c, request)
}

// DescribeZones
// 该接口用于查询您有权限的站点信息。可根据不同查询条件筛选站点。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func DescribeZonesWithContext(ctx context.Context, c *Client, request *DescribeZonesRequest) (response *DescribeZonesResponse, err error) {
    if request == nil {
        request = NewDescribeZonesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DescribeZones")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DescribeZones require credential")
    }

    request.SetContext(ctx)
    
    response = NewDescribeZonesResponse()
    err = c.Send(request, response)
    return
}

func NewDestroyPlanRequest() (request *DestroyPlanRequest) {
    request = &DestroyPlanRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DestroyPlan")
    
    
    return
}

func NewDestroyPlanResponse() (response *DestroyPlanResponse) {
    response = &DestroyPlanResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DestroyPlan
// 当您需要停止 Edgeone 套餐的计费，可以通过该接口销毁计费套餐。
//
// > 销毁计费套餐需要满足以下条件：
//
//     1.套餐已过期（企业版套餐除外）；
//
//     2.套餐下所有站点均已关闭或删除。
//
// 
//
// > 站点状态可以通过 [查询站点列表](https://cloud.tencent.com/document/product/1552/80713) 接口进行查询
//
// 停用站点可以通过 [切换站点状态](https://cloud.tencent.com/document/product/1552/80707) 接口将站点切换至关闭状态
//
// 删除站点可以通过 [删除站点](https://cloud.tencent.com/document/product/1552/80717) 接口将站点删除
//
// 可能返回的错误码:
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
func DestroyPlan(c *Client, request *DestroyPlanRequest) (response *DestroyPlanResponse, err error) {
    return DestroyPlanWithContext(context.Background(), c, request)
}

// DestroyPlan
// 当您需要停止 Edgeone 套餐的计费，可以通过该接口销毁计费套餐。
//
// > 销毁计费套餐需要满足以下条件：
//
//     1.套餐已过期（企业版套餐除外）；
//
//     2.套餐下所有站点均已关闭或删除。
//
// 
//
// > 站点状态可以通过 [查询站点列表](https://cloud.tencent.com/document/product/1552/80713) 接口进行查询
//
// 停用站点可以通过 [切换站点状态](https://cloud.tencent.com/document/product/1552/80707) 接口将站点切换至关闭状态
//
// 删除站点可以通过 [删除站点](https://cloud.tencent.com/document/product/1552/80717) 接口将站点删除
//
// 可能返回的错误码:
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
func DestroyPlanWithContext(ctx context.Context, c *Client, request *DestroyPlanRequest) (response *DestroyPlanResponse, err error) {
    if request == nil {
        request = NewDestroyPlanRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DestroyPlan")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DestroyPlan require credential")
    }

    request.SetContext(ctx)
    
    response = NewDestroyPlanResponse()
    err = c.Send(request, response)
    return
}

func NewDisableOriginACLRequest() (request *DisableOriginACLRequest) {
    request = &DisableOriginACLRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DisableOriginACL")
    
    
    return
}

func NewDisableOriginACLResponse() (response *DisableOriginACLResponse) {
    response = &DisableOriginACLResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DisableOriginACL
// 本接口用于关闭站点的源站防护功能。停用后，相关资源不再仅使用「源站防护」提供的回源 IP 网段请求您的源站，同时停止发送回源 IP 网段更新通知。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR = "InternalError"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DisableOriginACL(c *Client, request *DisableOriginACLRequest) (response *DisableOriginACLResponse, err error) {
    return DisableOriginACLWithContext(context.Background(), c, request)
}

// DisableOriginACL
// 本接口用于关闭站点的源站防护功能。停用后，相关资源不再仅使用「源站防护」提供的回源 IP 网段请求您的源站，同时停止发送回源 IP 网段更新通知。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR = "InternalError"
//  OPERATIONDENIED = "OperationDenied"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DisableOriginACLWithContext(ctx context.Context, c *Client, request *DisableOriginACLRequest) (response *DisableOriginACLResponse, err error) {
    if request == nil {
        request = NewDisableOriginACLRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DisableOriginACL")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DisableOriginACL require credential")
    }

    request.SetContext(ctx)
    
    response = NewDisableOriginACLResponse()
    err = c.Send(request, response)
    return
}

func NewDownloadL4LogsRequest() (request *DownloadL4LogsRequest) {
    request = &DownloadL4LogsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DownloadL4Logs")
    
    
    return
}

func NewDownloadL4LogsResponse() (response *DownloadL4LogsResponse) {
    response = &DownloadL4LogsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DownloadL4Logs
// 本接口（DownloadL4Logs）用于下载四层离线日志。
//
// 可能返回的错误码:
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
func DownloadL4Logs(c *Client, request *DownloadL4LogsRequest) (response *DownloadL4LogsResponse, err error) {
    return DownloadL4LogsWithContext(context.Background(), c, request)
}

// DownloadL4Logs
// 本接口（DownloadL4Logs）用于下载四层离线日志。
//
// 可能返回的错误码:
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
func DownloadL4LogsWithContext(ctx context.Context, c *Client, request *DownloadL4LogsRequest) (response *DownloadL4LogsResponse, err error) {
    if request == nil {
        request = NewDownloadL4LogsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DownloadL4Logs")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DownloadL4Logs require credential")
    }

    request.SetContext(ctx)
    
    response = NewDownloadL4LogsResponse()
    err = c.Send(request, response)
    return
}

func NewDownloadL7LogsRequest() (request *DownloadL7LogsRequest) {
    request = &DownloadL7LogsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "DownloadL7Logs")
    
    
    return
}

func NewDownloadL7LogsResponse() (response *DownloadL7LogsResponse) {
    response = &DownloadL7LogsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// DownloadL7Logs
// 本接口（DownloadL7Logs）下载七层离线日志。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DownloadL7Logs(c *Client, request *DownloadL7LogsRequest) (response *DownloadL7LogsResponse, err error) {
    return DownloadL7LogsWithContext(context.Background(), c, request)
}

// DownloadL7Logs
// 本接口（DownloadL7Logs）下载七层离线日志。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func DownloadL7LogsWithContext(ctx context.Context, c *Client, request *DownloadL7LogsRequest) (response *DownloadL7LogsResponse, err error) {
    if request == nil {
        request = NewDownloadL7LogsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "DownloadL7Logs")
    
    if c.GetCredential() == nil {
        return nil, errors.New("DownloadL7Logs require credential")
    }

    request.SetContext(ctx)
    
    response = NewDownloadL7LogsResponse()
    err = c.Send(request, response)
    return
}

func NewEnableOriginACLRequest() (request *EnableOriginACLRequest) {
    request = &EnableOriginACLRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "EnableOriginACL")
    
    
    return
}

func NewEnableOriginACLResponse() (response *EnableOriginACLResponse) {
    response = &EnableOriginACLResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// EnableOriginACL
// 本接口用于站点首次开启源站防护，启用后 EdgeOne 将会使用特定的回源 IP 网段为七层加速域名/四层代理实例回源。单次支持提交的七层加速域名的数量最大为 200，四层代理实例的数量最大为 100，支持七层加速域名/四层代理实例混合提交，总实例个数最大为 200。如需要启用超过 200 个资源，可先通过指定资源的方式以最大数量启用，剩余资源通过 ModifyOriginACL 接口启用。后续新增七层加速域名/四层代理实例均请通过 ModifyOriginACL 接口配置。
//
// 
//
// 注意：
//
// - 调用本接口视为同意 [源站防护启用特别约定](https://cloud.tencent.com/document/product/1552/120141)；
//
// - 回源 IP 网段会不定期变更，EdgeOne 将在回源 IP 网段变更前 14 天、前 7 天、前 3 天和前 1 天分别通过站内信、短信、邮件等一种或多种方式发起通知，为了能正常收到回源 IP 网段的变更通知，请务必确保您在 [腾讯云消息中心控制台](https://console.cloud.tencent.com/message)内，已勾选边缘安全加速平台 EO 的产品服务相关消息通知，并配置正确的消息接收人。配置方式请参考 [消息订阅管理](https://cloud.tencent.com/document/product/567/43476)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_INVALIDDOMAINS = "InvalidParameter.InvalidDomains"
//  INVALIDPARAMETER_INVALIDPROXIES = "InvalidParameter.InvalidProxies"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_UNSUPPORTEDPLAN = "OperationDenied.UnsupportedPlan"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func EnableOriginACL(c *Client, request *EnableOriginACLRequest) (response *EnableOriginACLResponse, err error) {
    return EnableOriginACLWithContext(context.Background(), c, request)
}

// EnableOriginACL
// 本接口用于站点首次开启源站防护，启用后 EdgeOne 将会使用特定的回源 IP 网段为七层加速域名/四层代理实例回源。单次支持提交的七层加速域名的数量最大为 200，四层代理实例的数量最大为 100，支持七层加速域名/四层代理实例混合提交，总实例个数最大为 200。如需要启用超过 200 个资源，可先通过指定资源的方式以最大数量启用，剩余资源通过 ModifyOriginACL 接口启用。后续新增七层加速域名/四层代理实例均请通过 ModifyOriginACL 接口配置。
//
// 
//
// 注意：
//
// - 调用本接口视为同意 [源站防护启用特别约定](https://cloud.tencent.com/document/product/1552/120141)；
//
// - 回源 IP 网段会不定期变更，EdgeOne 将在回源 IP 网段变更前 14 天、前 7 天、前 3 天和前 1 天分别通过站内信、短信、邮件等一种或多种方式发起通知，为了能正常收到回源 IP 网段的变更通知，请务必确保您在 [腾讯云消息中心控制台](https://console.cloud.tencent.com/message)内，已勾选边缘安全加速平台 EO 的产品服务相关消息通知，并配置正确的消息接收人。配置方式请参考 [消息订阅管理](https://cloud.tencent.com/document/product/567/43476)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_INVALIDDOMAINS = "InvalidParameter.InvalidDomains"
//  INVALIDPARAMETER_INVALIDPROXIES = "InvalidParameter.InvalidProxies"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_UNSUPPORTEDPLAN = "OperationDenied.UnsupportedPlan"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func EnableOriginACLWithContext(ctx context.Context, c *Client, request *EnableOriginACLRequest) (response *EnableOriginACLResponse, err error) {
    if request == nil {
        request = NewEnableOriginACLRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "EnableOriginACL")
    
    if c.GetCredential() == nil {
        return nil, errors.New("EnableOriginACL require credential")
    }

    request.SetContext(ctx)
    
    response = NewEnableOriginACLResponse()
    err = c.Send(request, response)
    return
}

func NewExportZoneConfigRequest() (request *ExportZoneConfigRequest) {
    request = &ExportZoneConfigRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ExportZoneConfig")
    
    
    return
}

func NewExportZoneConfigResponse() (response *ExportZoneConfigResponse) {
    response = &ExportZoneConfigResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ExportZoneConfig
// 导出站点配置接口，本接口支持用户根据需要的配置项进行配置导出，导出的配置用于导入站点配置接口（ImportZoneConfig）进行配置导入。该功能仅支持标准版和企业版套餐站点使用。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_INVALIDDOMAINS = "InvalidParameter.InvalidDomains"
//  INVALIDPARAMETER_INVALIDPROXIES = "InvalidParameter.InvalidProxies"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_UNSUPPORTEDPLAN = "OperationDenied.UnsupportedPlan"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func ExportZoneConfig(c *Client, request *ExportZoneConfigRequest) (response *ExportZoneConfigResponse, err error) {
    return ExportZoneConfigWithContext(context.Background(), c, request)
}

// ExportZoneConfig
// 导出站点配置接口，本接口支持用户根据需要的配置项进行配置导出，导出的配置用于导入站点配置接口（ImportZoneConfig）进行配置导入。该功能仅支持标准版和企业版套餐站点使用。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_INVALIDDOMAINS = "InvalidParameter.InvalidDomains"
//  INVALIDPARAMETER_INVALIDPROXIES = "InvalidParameter.InvalidProxies"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_UNSUPPORTEDPLAN = "OperationDenied.UnsupportedPlan"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func ExportZoneConfigWithContext(ctx context.Context, c *Client, request *ExportZoneConfigRequest) (response *ExportZoneConfigResponse, err error) {
    if request == nil {
        request = NewExportZoneConfigRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ExportZoneConfig")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ExportZoneConfig require credential")
    }

    request.SetContext(ctx)
    
    response = NewExportZoneConfigResponse()
    err = c.Send(request, response)
    return
}

func NewHandleFunctionRuntimeEnvironmentRequest() (request *HandleFunctionRuntimeEnvironmentRequest) {
    request = &HandleFunctionRuntimeEnvironmentRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "HandleFunctionRuntimeEnvironment")
    
    
    return
}

func NewHandleFunctionRuntimeEnvironmentResponse() (response *HandleFunctionRuntimeEnvironmentResponse) {
    response = &HandleFunctionRuntimeEnvironmentResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// HandleFunctionRuntimeEnvironment
// 操作边缘函数运行环境，支持环境变量的相关设置。
//
// 设置环境变量后，可在函数代码中使用，具体参考 [边缘函数引入环境变量](https://cloud.tencent.com/document/product/1552/109151#0151fd9a-8b0e-407b-ae37-54553a60ded6)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_FUNCTIONDEPLOYING = "FailedOperation.FunctionDeploying"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func HandleFunctionRuntimeEnvironment(c *Client, request *HandleFunctionRuntimeEnvironmentRequest) (response *HandleFunctionRuntimeEnvironmentResponse, err error) {
    return HandleFunctionRuntimeEnvironmentWithContext(context.Background(), c, request)
}

// HandleFunctionRuntimeEnvironment
// 操作边缘函数运行环境，支持环境变量的相关设置。
//
// 设置环境变量后，可在函数代码中使用，具体参考 [边缘函数引入环境变量](https://cloud.tencent.com/document/product/1552/109151#0151fd9a-8b0e-407b-ae37-54553a60ded6)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_FUNCTIONDEPLOYING = "FailedOperation.FunctionDeploying"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func HandleFunctionRuntimeEnvironmentWithContext(ctx context.Context, c *Client, request *HandleFunctionRuntimeEnvironmentRequest) (response *HandleFunctionRuntimeEnvironmentResponse, err error) {
    if request == nil {
        request = NewHandleFunctionRuntimeEnvironmentRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "HandleFunctionRuntimeEnvironment")
    
    if c.GetCredential() == nil {
        return nil, errors.New("HandleFunctionRuntimeEnvironment require credential")
    }

    request.SetContext(ctx)
    
    response = NewHandleFunctionRuntimeEnvironmentResponse()
    err = c.Send(request, response)
    return
}

func NewIdentifyZoneRequest() (request *IdentifyZoneRequest) {
    request = &IdentifyZoneRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "IdentifyZone")
    
    
    return
}

func NewIdentifyZoneResponse() (response *IdentifyZoneResponse) {
    response = &IdentifyZoneResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// IdentifyZone
// 用于验证站点所有权。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func IdentifyZone(c *Client, request *IdentifyZoneRequest) (response *IdentifyZoneResponse, err error) {
    return IdentifyZoneWithContext(context.Background(), c, request)
}

// IdentifyZone
// 用于验证站点所有权。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func IdentifyZoneWithContext(ctx context.Context, c *Client, request *IdentifyZoneRequest) (response *IdentifyZoneResponse, err error) {
    if request == nil {
        request = NewIdentifyZoneRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "IdentifyZone")
    
    if c.GetCredential() == nil {
        return nil, errors.New("IdentifyZone require credential")
    }

    request.SetContext(ctx)
    
    response = NewIdentifyZoneResponse()
    err = c.Send(request, response)
    return
}

func NewImportZoneConfigRequest() (request *ImportZoneConfigRequest) {
    request = &ImportZoneConfigRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ImportZoneConfig")
    
    
    return
}

func NewImportZoneConfigResponse() (response *ImportZoneConfigResponse) {
    response = &ImportZoneConfigResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ImportZoneConfig
// 导入站点配置接口，本接口支持站点配置文件的快速导入，发起导入后接口会返回对应的任务 ID（TaskId），用户需通过查询站点配置导入结果接口（DescribeZoneConfigImportResult）获取本次导入任务执行的结果。该功能仅支持标准版和企业版套餐站点使用。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func ImportZoneConfig(c *Client, request *ImportZoneConfigRequest) (response *ImportZoneConfigResponse, err error) {
    return ImportZoneConfigWithContext(context.Background(), c, request)
}

// ImportZoneConfig
// 导入站点配置接口，本接口支持站点配置文件的快速导入，发起导入后接口会返回对应的任务 ID（TaskId），用户需通过查询站点配置导入结果接口（DescribeZoneConfigImportResult）获取本次导入任务执行的结果。该功能仅支持标准版和企业版套餐站点使用。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func ImportZoneConfigWithContext(ctx context.Context, c *Client, request *ImportZoneConfigRequest) (response *ImportZoneConfigResponse, err error) {
    if request == nil {
        request = NewImportZoneConfigRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ImportZoneConfig")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ImportZoneConfig require credential")
    }

    request.SetContext(ctx)
    
    response = NewImportZoneConfigResponse()
    err = c.Send(request, response)
    return
}

func NewIncreasePlanQuotaRequest() (request *IncreasePlanQuotaRequest) {
    request = &IncreasePlanQuotaRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "IncreasePlanQuota")
    
    
    return
}

func NewIncreasePlanQuotaResponse() (response *IncreasePlanQuotaResponse) {
    response = &IncreasePlanQuotaResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// IncreasePlanQuota
// 当您的套餐绑定的站点数，或配置的 Web 防护 - 自定义规则 - 精准匹配策略的规则数，或 Web 防护 - 速率限制 - 精准速率限制模块的规则数达到套餐允许的配额上限，可以通过该接口增购对应配额。
//
// > 该接口该仅支持企业版套餐。
//
// 可能返回的错误码:
//  FAILEDOPERATION_INSUFFICIENTACCOUNTBALANCE = "FailedOperation.InsufficientAccountBalance"
//  INVALIDPARAMETER_INVALIDQUOTANUMBER = "InvalidParameter.InvalidQuotaNumber"
//  INVALIDPARAMETER_INVALIDQUOTATYPE = "InvalidParameter.InvalidQuotaType"
//  OPERATIONDENIED_PLANHASBEENISOLATED = "OperationDenied.PlanHasBeenIsolated"
//  OPERATIONDENIED_PLANINCREASEPLANQUOTAUNSUPPORTED = "OperationDenied.PlanIncreasePlanQuotaUnsupported"
//  OPERATIONDENIED_RESOURCEHASBEENLOCKED = "OperationDenied.ResourceHasBeenLocked"
func IncreasePlanQuota(c *Client, request *IncreasePlanQuotaRequest) (response *IncreasePlanQuotaResponse, err error) {
    return IncreasePlanQuotaWithContext(context.Background(), c, request)
}

// IncreasePlanQuota
// 当您的套餐绑定的站点数，或配置的 Web 防护 - 自定义规则 - 精准匹配策略的规则数，或 Web 防护 - 速率限制 - 精准速率限制模块的规则数达到套餐允许的配额上限，可以通过该接口增购对应配额。
//
// > 该接口该仅支持企业版套餐。
//
// 可能返回的错误码:
//  FAILEDOPERATION_INSUFFICIENTACCOUNTBALANCE = "FailedOperation.InsufficientAccountBalance"
//  INVALIDPARAMETER_INVALIDQUOTANUMBER = "InvalidParameter.InvalidQuotaNumber"
//  INVALIDPARAMETER_INVALIDQUOTATYPE = "InvalidParameter.InvalidQuotaType"
//  OPERATIONDENIED_PLANHASBEENISOLATED = "OperationDenied.PlanHasBeenIsolated"
//  OPERATIONDENIED_PLANINCREASEPLANQUOTAUNSUPPORTED = "OperationDenied.PlanIncreasePlanQuotaUnsupported"
//  OPERATIONDENIED_RESOURCEHASBEENLOCKED = "OperationDenied.ResourceHasBeenLocked"
func IncreasePlanQuotaWithContext(ctx context.Context, c *Client, request *IncreasePlanQuotaRequest) (response *IncreasePlanQuotaResponse, err error) {
    if request == nil {
        request = NewIncreasePlanQuotaRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "IncreasePlanQuota")
    
    if c.GetCredential() == nil {
        return nil, errors.New("IncreasePlanQuota require credential")
    }

    request.SetContext(ctx)
    
    response = NewIncreasePlanQuotaResponse()
    err = c.Send(request, response)
    return
}

func NewModifyAccelerationDomainRequest() (request *ModifyAccelerationDomainRequest) {
    request = &ModifyAccelerationDomainRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyAccelerationDomain")
    
    
    return
}

func NewModifyAccelerationDomainResponse() (response *ModifyAccelerationDomainResponse) {
    response = &ModifyAccelerationDomainResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyAccelerationDomain
// 修改加速域名信息
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CONFLICTHOSTORIGIN = "InvalidParameter.ConflictHostOrigin"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDAWSSECRETKEY = "InvalidParameter.InvalidAwsSecretKey"
//  INVALIDPARAMETER_INVALIDCLIENTIPORIGIN = "InvalidParameter.InvalidClientIpOrigin"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_INVALIDPRIVATEACCESSPARAMS = "InvalidParameter.InvalidPrivateAccessParams"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGCLIENTIP = "InvalidParameter.InvalidStandardDebugClientIp"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_ORIGINISINNERIP = "InvalidParameter.OriginIsInnerIp"
//  INVALIDPARAMETER_SPACENOTBINDORIGIN = "InvalidParameter.SpaceNotBindOrigin"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETERVALUE_CONFLICTRECORD = "InvalidParameterValue.ConflictRecord"
//  INVALIDPARAMETERVALUE_DOMAINNOTMATCHZONE = "InvalidParameterValue.DomainNotMatchZone"
//  INVALIDPARAMETERVALUE_INVALIDDOMAINSTATUS = "InvalidParameterValue.InvalidDomainStatus"
//  OPERATIONDENIED_DOMAINNOICP = "OperationDenied.DomainNoICP"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEINUSE_DNSRECORD = "ResourceInUse.DnsRecord"
//  RESOURCEUNAVAILABLE_DOMAINNOTFOUND = "ResourceUnavailable.DomainNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func ModifyAccelerationDomain(c *Client, request *ModifyAccelerationDomainRequest) (response *ModifyAccelerationDomainResponse, err error) {
    return ModifyAccelerationDomainWithContext(context.Background(), c, request)
}

// ModifyAccelerationDomain
// 修改加速域名信息
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CONFLICTHOSTORIGIN = "InvalidParameter.ConflictHostOrigin"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDAWSSECRETKEY = "InvalidParameter.InvalidAwsSecretKey"
//  INVALIDPARAMETER_INVALIDCLIENTIPORIGIN = "InvalidParameter.InvalidClientIpOrigin"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_INVALIDPRIVATEACCESSPARAMS = "InvalidParameter.InvalidPrivateAccessParams"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGCLIENTIP = "InvalidParameter.InvalidStandardDebugClientIp"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_ORIGINISINNERIP = "InvalidParameter.OriginIsInnerIp"
//  INVALIDPARAMETER_SPACENOTBINDORIGIN = "InvalidParameter.SpaceNotBindOrigin"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETERVALUE_CONFLICTRECORD = "InvalidParameterValue.ConflictRecord"
//  INVALIDPARAMETERVALUE_DOMAINNOTMATCHZONE = "InvalidParameterValue.DomainNotMatchZone"
//  INVALIDPARAMETERVALUE_INVALIDDOMAINSTATUS = "InvalidParameterValue.InvalidDomainStatus"
//  OPERATIONDENIED_DOMAINNOICP = "OperationDenied.DomainNoICP"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEINUSE_DNSRECORD = "ResourceInUse.DnsRecord"
//  RESOURCEUNAVAILABLE_DOMAINNOTFOUND = "ResourceUnavailable.DomainNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func ModifyAccelerationDomainWithContext(ctx context.Context, c *Client, request *ModifyAccelerationDomainRequest) (response *ModifyAccelerationDomainResponse, err error) {
    if request == nil {
        request = NewModifyAccelerationDomainRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyAccelerationDomain")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyAccelerationDomain require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyAccelerationDomainResponse()
    err = c.Send(request, response)
    return
}

func NewModifyAccelerationDomainStatusesRequest() (request *ModifyAccelerationDomainStatusesRequest) {
    request = &ModifyAccelerationDomainStatusesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyAccelerationDomainStatuses")
    
    
    return
}

func NewModifyAccelerationDomainStatusesResponse() (response *ModifyAccelerationDomainStatusesResponse) {
    response = &ModifyAccelerationDomainStatusesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyAccelerationDomainStatuses
// 批量修改加速域名状态
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDERRORPAGE = "InvalidParameter.InvalidErrorPage"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETERVALUE_DOMAINNOTMATCHZONE = "InvalidParameterValue.DomainNotMatchZone"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEUNAVAILABLE_DOMAINNOTFOUND = "ResourceUnavailable.DomainNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifyAccelerationDomainStatuses(c *Client, request *ModifyAccelerationDomainStatusesRequest) (response *ModifyAccelerationDomainStatusesResponse, err error) {
    return ModifyAccelerationDomainStatusesWithContext(context.Background(), c, request)
}

// ModifyAccelerationDomainStatuses
// 批量修改加速域名状态
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_INVALIDERRORPAGE = "InvalidParameter.InvalidErrorPage"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETERVALUE_DOMAINNOTMATCHZONE = "InvalidParameterValue.DomainNotMatchZone"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  OPERATIONDENIED_VERSIONCONTROLLOCKED = "OperationDenied.VersionControlLocked"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEUNAVAILABLE_DOMAINNOTFOUND = "ResourceUnavailable.DomainNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifyAccelerationDomainStatusesWithContext(ctx context.Context, c *Client, request *ModifyAccelerationDomainStatusesRequest) (response *ModifyAccelerationDomainStatusesResponse, err error) {
    if request == nil {
        request = NewModifyAccelerationDomainStatusesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyAccelerationDomainStatuses")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyAccelerationDomainStatuses require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyAccelerationDomainStatusesResponse()
    err = c.Send(request, response)
    return
}

func NewModifyAliasDomainRequest() (request *ModifyAliasDomainRequest) {
    request = &ModifyAliasDomainRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyAliasDomain")
    
    
    return
}

func NewModifyAliasDomainResponse() (response *ModifyAliasDomainResponse) {
    response = &ModifyAliasDomainResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyAliasDomain
// 修改别称域名。
//
// 该功能仅企业版套餐支持，并且该功能当前仍在内测中，如需使用，请[联系我们](https://cloud.tencent.com/online-service?from=connect-us)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER_CERTNOTMATCHDOMAIN = "InvalidParameter.CertNotMatchDomain"
//  OPERATIONDENIED = "OperationDenied"
func ModifyAliasDomain(c *Client, request *ModifyAliasDomainRequest) (response *ModifyAliasDomainResponse, err error) {
    return ModifyAliasDomainWithContext(context.Background(), c, request)
}

// ModifyAliasDomain
// 修改别称域名。
//
// 该功能仅企业版套餐支持，并且该功能当前仍在内测中，如需使用，请[联系我们](https://cloud.tencent.com/online-service?from=connect-us)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER_CERTNOTMATCHDOMAIN = "InvalidParameter.CertNotMatchDomain"
//  OPERATIONDENIED = "OperationDenied"
func ModifyAliasDomainWithContext(ctx context.Context, c *Client, request *ModifyAliasDomainRequest) (response *ModifyAliasDomainResponse, err error) {
    if request == nil {
        request = NewModifyAliasDomainRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyAliasDomain")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyAliasDomain require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyAliasDomainResponse()
    err = c.Send(request, response)
    return
}

func NewModifyAliasDomainStatusRequest() (request *ModifyAliasDomainStatusRequest) {
    request = &ModifyAliasDomainStatusRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyAliasDomainStatus")
    
    
    return
}

func NewModifyAliasDomainStatusResponse() (response *ModifyAliasDomainStatusResponse) {
    response = &ModifyAliasDomainStatusResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyAliasDomainStatus
// 修改别称域名状态。
//
// 该功能仅企业版套餐支持，并且该功能当前仍在内测中，如需使用，请[联系我们](https://cloud.tencent.com/online-service?from=connect-us)。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEINUSE = "ResourceInUse"
func ModifyAliasDomainStatus(c *Client, request *ModifyAliasDomainStatusRequest) (response *ModifyAliasDomainStatusResponse, err error) {
    return ModifyAliasDomainStatusWithContext(context.Background(), c, request)
}

// ModifyAliasDomainStatus
// 修改别称域名状态。
//
// 该功能仅企业版套餐支持，并且该功能当前仍在内测中，如需使用，请[联系我们](https://cloud.tencent.com/online-service?from=connect-us)。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEINUSE = "ResourceInUse"
func ModifyAliasDomainStatusWithContext(ctx context.Context, c *Client, request *ModifyAliasDomainStatusRequest) (response *ModifyAliasDomainStatusResponse, err error) {
    if request == nil {
        request = NewModifyAliasDomainStatusRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyAliasDomainStatus")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyAliasDomainStatus require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyAliasDomainStatusResponse()
    err = c.Send(request, response)
    return
}

func NewModifyApplicationProxyRequest() (request *ModifyApplicationProxyRequest) {
    request = &ModifyApplicationProxyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyApplicationProxy")
    
    
    return
}

func NewModifyApplicationProxyResponse() (response *ModifyApplicationProxyResponse) {
    response = &ModifyApplicationProxyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyApplicationProxy
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [修改四层代理实例
//
// ](https://cloud.tencent.com/document/product/1552/103411) 。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4LACKOFRESOURCES = "OperationDenied.L4LackOfResources"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyApplicationProxy(c *Client, request *ModifyApplicationProxyRequest) (response *ModifyApplicationProxyResponse, err error) {
    return ModifyApplicationProxyWithContext(context.Background(), c, request)
}

// ModifyApplicationProxy
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [修改四层代理实例
//
// ](https://cloud.tencent.com/document/product/1552/103411) 。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4LACKOFRESOURCES = "OperationDenied.L4LackOfResources"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyApplicationProxyWithContext(ctx context.Context, c *Client, request *ModifyApplicationProxyRequest) (response *ModifyApplicationProxyResponse, err error) {
    if request == nil {
        request = NewModifyApplicationProxyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyApplicationProxy")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyApplicationProxy require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyApplicationProxyResponse()
    err = c.Send(request, response)
    return
}

func NewModifyApplicationProxyRuleRequest() (request *ModifyApplicationProxyRuleRequest) {
    request = &ModifyApplicationProxyRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyApplicationProxyRule")
    
    
    return
}

func NewModifyApplicationProxyRuleResponse() (response *ModifyApplicationProxyRuleResponse) {
    response = &ModifyApplicationProxyRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyApplicationProxyRule
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [修改四层代理转发规则
//
// ](https://cloud.tencent.com/document/product/1552/103410) 。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyApplicationProxyRule(c *Client, request *ModifyApplicationProxyRuleRequest) (response *ModifyApplicationProxyRuleResponse, err error) {
    return ModifyApplicationProxyRuleWithContext(context.Background(), c, request)
}

// ModifyApplicationProxyRule
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [修改四层代理转发规则
//
// ](https://cloud.tencent.com/document/product/1552/103410) 。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyApplicationProxyRuleWithContext(ctx context.Context, c *Client, request *ModifyApplicationProxyRuleRequest) (response *ModifyApplicationProxyRuleResponse, err error) {
    if request == nil {
        request = NewModifyApplicationProxyRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyApplicationProxyRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyApplicationProxyRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyApplicationProxyRuleResponse()
    err = c.Send(request, response)
    return
}

func NewModifyApplicationProxyRuleStatusRequest() (request *ModifyApplicationProxyRuleStatusRequest) {
    request = &ModifyApplicationProxyRuleStatusRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyApplicationProxyRuleStatus")
    
    
    return
}

func NewModifyApplicationProxyRuleStatusResponse() (response *ModifyApplicationProxyRuleStatusResponse) {
    response = &ModifyApplicationProxyRuleStatusResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyApplicationProxyRuleStatus
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [修改四层代理转发规则状态
//
// ](https://cloud.tencent.com/document/product/1552/103409) 。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyApplicationProxyRuleStatus(c *Client, request *ModifyApplicationProxyRuleStatusRequest) (response *ModifyApplicationProxyRuleStatusResponse, err error) {
    return ModifyApplicationProxyRuleStatusWithContext(context.Background(), c, request)
}

// ModifyApplicationProxyRuleStatus
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [修改四层代理转发规则状态
//
// ](https://cloud.tencent.com/document/product/1552/103409) 。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyApplicationProxyRuleStatusWithContext(ctx context.Context, c *Client, request *ModifyApplicationProxyRuleStatusRequest) (response *ModifyApplicationProxyRuleStatusResponse, err error) {
    if request == nil {
        request = NewModifyApplicationProxyRuleStatusRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyApplicationProxyRuleStatus")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyApplicationProxyRuleStatus require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyApplicationProxyRuleStatusResponse()
    err = c.Send(request, response)
    return
}

func NewModifyApplicationProxyStatusRequest() (request *ModifyApplicationProxyStatusRequest) {
    request = &ModifyApplicationProxyStatusRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyApplicationProxyStatus")
    
    
    return
}

func NewModifyApplicationProxyStatusResponse() (response *ModifyApplicationProxyStatusResponse) {
    response = &ModifyApplicationProxyStatusResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyApplicationProxyStatus
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [修改四层代理实例状态](https://cloud.tencent.com/document/product/1552/103408) 。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyApplicationProxyStatus(c *Client, request *ModifyApplicationProxyStatusRequest) (response *ModifyApplicationProxyStatusResponse, err error) {
    return ModifyApplicationProxyStatusWithContext(context.Background(), c, request)
}

// ModifyApplicationProxyStatus
// 本接口为旧版，如需调用请尽快迁移至新版，详情请参考 [修改四层代理实例状态](https://cloud.tencent.com/document/product/1552/103408) 。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyApplicationProxyStatusWithContext(ctx context.Context, c *Client, request *ModifyApplicationProxyStatusRequest) (response *ModifyApplicationProxyStatusResponse, err error) {
    if request == nil {
        request = NewModifyApplicationProxyStatusRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyApplicationProxyStatus")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyApplicationProxyStatus require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyApplicationProxyStatusResponse()
    err = c.Send(request, response)
    return
}

func NewModifyContentIdentifierRequest() (request *ModifyContentIdentifierRequest) {
    request = &ModifyContentIdentifierRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyContentIdentifier")
    
    
    return
}

func NewModifyContentIdentifierResponse() (response *ModifyContentIdentifierResponse) {
    response = &ModifyContentIdentifierResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyContentIdentifier
// 修改内容标识符，仅支持修改描述。该功能仅白名单开放。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyContentIdentifier(c *Client, request *ModifyContentIdentifierRequest) (response *ModifyContentIdentifierResponse, err error) {
    return ModifyContentIdentifierWithContext(context.Background(), c, request)
}

// ModifyContentIdentifier
// 修改内容标识符，仅支持修改描述。该功能仅白名单开放。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyContentIdentifierWithContext(ctx context.Context, c *Client, request *ModifyContentIdentifierRequest) (response *ModifyContentIdentifierResponse, err error) {
    if request == nil {
        request = NewModifyContentIdentifierRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyContentIdentifier")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyContentIdentifier require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyContentIdentifierResponse()
    err = c.Send(request, response)
    return
}

func NewModifyCustomErrorPageRequest() (request *ModifyCustomErrorPageRequest) {
    request = &ModifyCustomErrorPageRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyCustomErrorPage")
    
    
    return
}

func NewModifyCustomErrorPageResponse() (response *ModifyCustomErrorPageResponse) {
    response = &ModifyCustomErrorPageResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyCustomErrorPage
// 修改自定义错误页面。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyCustomErrorPage(c *Client, request *ModifyCustomErrorPageRequest) (response *ModifyCustomErrorPageResponse, err error) {
    return ModifyCustomErrorPageWithContext(context.Background(), c, request)
}

// ModifyCustomErrorPage
// 修改自定义错误页面。
//
// 可能返回的错误码:
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyCustomErrorPageWithContext(ctx context.Context, c *Client, request *ModifyCustomErrorPageRequest) (response *ModifyCustomErrorPageResponse, err error) {
    if request == nil {
        request = NewModifyCustomErrorPageRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyCustomErrorPage")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyCustomErrorPage require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyCustomErrorPageResponse()
    err = c.Send(request, response)
    return
}

func NewModifyDDoSProtectionRequest() (request *ModifyDDoSProtectionRequest) {
    request = &ModifyDDoSProtectionRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyDDoSProtection")
    
    
    return
}

func NewModifyDDoSProtectionResponse() (response *ModifyDDoSProtectionResponse) {
    response = &ModifyDDoSProtectionResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyDDoSProtection
// 修改站点的独立 DDoS 防护。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED_SECURITYLACKOFRESOURCES = "OperationDenied.SecurityLackOfResources"
func ModifyDDoSProtection(c *Client, request *ModifyDDoSProtectionRequest) (response *ModifyDDoSProtectionResponse, err error) {
    return ModifyDDoSProtectionWithContext(context.Background(), c, request)
}

// ModifyDDoSProtection
// 修改站点的独立 DDoS 防护。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED_SECURITYLACKOFRESOURCES = "OperationDenied.SecurityLackOfResources"
func ModifyDDoSProtectionWithContext(ctx context.Context, c *Client, request *ModifyDDoSProtectionRequest) (response *ModifyDDoSProtectionResponse, err error) {
    if request == nil {
        request = NewModifyDDoSProtectionRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyDDoSProtection")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyDDoSProtection require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyDDoSProtectionResponse()
    err = c.Send(request, response)
    return
}

func NewModifyDnsRecordsRequest() (request *ModifyDnsRecordsRequest) {
    request = &ModifyDnsRecordsRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyDnsRecords")
    
    
    return
}

func NewModifyDnsRecordsResponse() (response *ModifyDnsRecordsResponse) {
    response = &ModifyDnsRecordsResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyDnsRecords
// 您可以通过本接口批量修改 DNS 记录。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED_SECURITYLACKOFRESOURCES = "OperationDenied.SecurityLackOfResources"
func ModifyDnsRecords(c *Client, request *ModifyDnsRecordsRequest) (response *ModifyDnsRecordsResponse, err error) {
    return ModifyDnsRecordsWithContext(context.Background(), c, request)
}

// ModifyDnsRecords
// 您可以通过本接口批量修改 DNS 记录。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED_SECURITYLACKOFRESOURCES = "OperationDenied.SecurityLackOfResources"
func ModifyDnsRecordsWithContext(ctx context.Context, c *Client, request *ModifyDnsRecordsRequest) (response *ModifyDnsRecordsResponse, err error) {
    if request == nil {
        request = NewModifyDnsRecordsRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyDnsRecords")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyDnsRecords require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyDnsRecordsResponse()
    err = c.Send(request, response)
    return
}

func NewModifyDnsRecordsStatusRequest() (request *ModifyDnsRecordsStatusRequest) {
    request = &ModifyDnsRecordsStatusRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyDnsRecordsStatus")
    
    
    return
}

func NewModifyDnsRecordsStatusResponse() (response *ModifyDnsRecordsStatusResponse) {
    response = &ModifyDnsRecordsStatusResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyDnsRecordsStatus
// 您可以通过本接口批量修改 DNS 记录的状态，批量对记录进行开启和停用。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED_SECURITYLACKOFRESOURCES = "OperationDenied.SecurityLackOfResources"
func ModifyDnsRecordsStatus(c *Client, request *ModifyDnsRecordsStatusRequest) (response *ModifyDnsRecordsStatusResponse, err error) {
    return ModifyDnsRecordsStatusWithContext(context.Background(), c, request)
}

// ModifyDnsRecordsStatus
// 您可以通过本接口批量修改 DNS 记录的状态，批量对记录进行开启和停用。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  OPERATIONDENIED_SECURITYLACKOFRESOURCES = "OperationDenied.SecurityLackOfResources"
func ModifyDnsRecordsStatusWithContext(ctx context.Context, c *Client, request *ModifyDnsRecordsStatusRequest) (response *ModifyDnsRecordsStatusResponse, err error) {
    if request == nil {
        request = NewModifyDnsRecordsStatusRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyDnsRecordsStatus")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyDnsRecordsStatus require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyDnsRecordsStatusResponse()
    err = c.Send(request, response)
    return
}

func NewModifyFunctionRequest() (request *ModifyFunctionRequest) {
    request = &ModifyFunctionRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyFunction")
    
    
    return
}

func NewModifyFunctionResponse() (response *ModifyFunctionResponse) {
    response = &ModifyFunctionResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyFunction
// 修改边缘函数，支持修改函数的内容及描述信息，修改且重新部署后，函数立刻生效。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_FUNCTIONDEPLOYING = "FailedOperation.FunctionDeploying"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_BADCONTENT = "InvalidParameter.BadContent"
//  INVALIDPARAMETER_CONTENTEXCEEDSLIMIT = "InvalidParameter.ContentExceedsLimit"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  INVALIDPARAMETER_MODIFYPARAMETERSMISSING = "InvalidParameter.ModifyParametersMissing"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func ModifyFunction(c *Client, request *ModifyFunctionRequest) (response *ModifyFunctionResponse, err error) {
    return ModifyFunctionWithContext(context.Background(), c, request)
}

// ModifyFunction
// 修改边缘函数，支持修改函数的内容及描述信息，修改且重新部署后，函数立刻生效。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_FUNCTIONDEPLOYING = "FailedOperation.FunctionDeploying"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_BADCONTENT = "InvalidParameter.BadContent"
//  INVALIDPARAMETER_CONTENTEXCEEDSLIMIT = "InvalidParameter.ContentExceedsLimit"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  INVALIDPARAMETER_MODIFYPARAMETERSMISSING = "InvalidParameter.ModifyParametersMissing"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func ModifyFunctionWithContext(ctx context.Context, c *Client, request *ModifyFunctionRequest) (response *ModifyFunctionResponse, err error) {
    if request == nil {
        request = NewModifyFunctionRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyFunction")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyFunction require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyFunctionResponse()
    err = c.Send(request, response)
    return
}

func NewModifyFunctionRuleRequest() (request *ModifyFunctionRuleRequest) {
    request = &ModifyFunctionRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyFunctionRule")
    
    
    return
}

func NewModifyFunctionRuleResponse() (response *ModifyFunctionRuleResponse) {
    response = &ModifyFunctionRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyFunctionRule
// 修改边缘函数触发规则，支持修改规则条件、执行函数以及描述信息。您可以先通过 DescribeFunctionRules 接口来获取需要修改的规则的 RuleId，然后传入修改后的规则内容，原规则内容会被覆盖式更新。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_RULEOPERATIONCONFLICT = "FailedOperation.RuleOperationConflict"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_DUPLICATERULE = "InvalidParameter.DuplicateRule"
//  INVALIDPARAMETER_INVALIDCONDITIONS = "InvalidParameter.InvalidConditions"
//  INVALIDPARAMETER_MODIFYPARAMETERSMISSING = "InvalidParameter.ModifyParametersMissing"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_RULENOTFOUND = "ResourceUnavailable.RuleNotFound"
func ModifyFunctionRule(c *Client, request *ModifyFunctionRuleRequest) (response *ModifyFunctionRuleResponse, err error) {
    return ModifyFunctionRuleWithContext(context.Background(), c, request)
}

// ModifyFunctionRule
// 修改边缘函数触发规则，支持修改规则条件、执行函数以及描述信息。您可以先通过 DescribeFunctionRules 接口来获取需要修改的规则的 RuleId，然后传入修改后的规则内容，原规则内容会被覆盖式更新。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_RULEOPERATIONCONFLICT = "FailedOperation.RuleOperationConflict"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_DUPLICATERULE = "InvalidParameter.DuplicateRule"
//  INVALIDPARAMETER_INVALIDCONDITIONS = "InvalidParameter.InvalidConditions"
//  INVALIDPARAMETER_MODIFYPARAMETERSMISSING = "InvalidParameter.ModifyParametersMissing"
//  RESOURCEUNAVAILABLE_FUNCTIONNOTFOUND = "ResourceUnavailable.FunctionNotFound"
//  RESOURCEUNAVAILABLE_RULENOTFOUND = "ResourceUnavailable.RuleNotFound"
func ModifyFunctionRuleWithContext(ctx context.Context, c *Client, request *ModifyFunctionRuleRequest) (response *ModifyFunctionRuleResponse, err error) {
    if request == nil {
        request = NewModifyFunctionRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyFunctionRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyFunctionRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyFunctionRuleResponse()
    err = c.Send(request, response)
    return
}

func NewModifyFunctionRulePriorityRequest() (request *ModifyFunctionRulePriorityRequest) {
    request = &ModifyFunctionRulePriorityRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyFunctionRulePriority")
    
    
    return
}

func NewModifyFunctionRulePriorityResponse() (response *ModifyFunctionRulePriorityResponse) {
    response = &ModifyFunctionRulePriorityResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyFunctionRulePriority
// 修改边缘函数触发规则的优先级。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_RULEOPERATIONCONFLICT = "FailedOperation.RuleOperationConflict"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func ModifyFunctionRulePriority(c *Client, request *ModifyFunctionRulePriorityRequest) (response *ModifyFunctionRulePriorityResponse, err error) {
    return ModifyFunctionRulePriorityWithContext(context.Background(), c, request)
}

// ModifyFunctionRulePriority
// 修改边缘函数触发规则的优先级。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_RULEOPERATIONCONFLICT = "FailedOperation.RuleOperationConflict"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
func ModifyFunctionRulePriorityWithContext(ctx context.Context, c *Client, request *ModifyFunctionRulePriorityRequest) (response *ModifyFunctionRulePriorityResponse, err error) {
    if request == nil {
        request = NewModifyFunctionRulePriorityRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyFunctionRulePriority")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyFunctionRulePriority require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyFunctionRulePriorityResponse()
    err = c.Send(request, response)
    return
}

func NewModifyHostsCertificateRequest() (request *ModifyHostsCertificateRequest) {
    request = &ModifyHostsCertificateRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyHostsCertificate")
    
    
    return
}

func NewModifyHostsCertificateResponse() (response *ModifyHostsCertificateResponse) {
    response = &ModifyHostsCertificateResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyHostsCertificate
// 完成域名创建之后，您可以为域名配置自有证书，也可以使用 EdgeOne 为您提供的 [免费证书](https://cloud.tencent.com/document/product/1552/90437)。
//
// 如果您需要配置自有证书，请先将证书上传至 [SSL证书控制台](https://console.cloud.tencent.com/certoverview)，然后在本接口中传入对应的证书 ID。详情参考 [部署自有证书至 EdgeOne 域名
//
// ](https://cloud.tencent.com/document/product/1552/88874)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CERTIFICATEHASEXPIRED = "FailedOperation.CertificateHasExpired"
//  FAILEDOPERATION_CERTIFICATENOTFOUND = "FailedOperation.CertificateNotFound"
//  FAILEDOPERATION_EDGECLIENTCERTIFICATEHASEXPIRED = "FailedOperation.EdgeClientCertificateHasExpired"
//  FAILEDOPERATION_INVALIDZONESTATUS = "FailedOperation.InvalidZoneStatus"
//  FAILEDOPERATION_MODIFYFAILED = "FailedOperation.ModifyFailed"
//  FAILEDOPERATION_UPSTREAMCLIENTCERTIFICATEHASEXPIRED = "FailedOperation.UpstreamClientCertificateHasExpired"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_GETROLEERROR = "InternalError.GetRoleError"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INTERNALERROR_UNKNOWERROR = "InternalError.UnknowError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ALIASDOMAINNOTSUPPORTKEYLESS = "InvalidParameter.AliasDomainNotSupportKeyless"
//  INVALIDPARAMETER_CERTNOTMATCHDOMAIN = "InvalidParameter.CertNotMatchDomain"
//  INVALIDPARAMETER_CERTTOEXPIRE = "InvalidParameter.CertToExpire"
//  INVALIDPARAMETER_CERTTOOSHORTKEYSIZE = "InvalidParameter.CertTooShortKeySize"
//  INVALIDPARAMETER_CERTIFICATECONFLICTWITHKEYLESSSERVER = "InvalidParameter.CertificateConflictWithKeylessServer"
//  INVALIDPARAMETER_CNAMEWILDHOSTNOTALLOWAPPLYCERTIFICATE = "InvalidParameter.CnameWildHostNotAllowApplyCertificate"
//  INVALIDPARAMETER_EDGECLIENTCERTCHECKERROR = "InvalidParameter.EdgeClientCertCheckError"
//  INVALIDPARAMETER_HOSTSTATUSNOTALLOWAPPLYCERTIFICATE = "InvalidParameter.HostStatusNotAllowApplyCertificate"
//  INVALIDPARAMETER_INVALIDCERTINFO = "InvalidParameter.InvalidCertInfo"
//  INVALIDPARAMETER_INVALIDHTTPSCERTINFO = "InvalidParameter.InvalidHttpsCertInfo"
//  INVALIDPARAMETER_INVALIDHTTPSTLSVERSION = "InvalidParameter.InvalidHttpsTlsVersion"
//  INVALIDPARAMETER_UPSTREAMCLIENTCERTCHECKERROR = "InvalidParameter.UpstreamClientCertCheckError"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETERVALUE_ALIASDOMAINNOTSUPPORTEDGEMTLS = "InvalidParameterValue.AliasDomainNotSupportEdgeMTLS"
//  INVALIDPARAMETERVALUE_ALIASDOMAINNOTSUPPORTUPSTREAMMTLS = "InvalidParameterValue.AliasDomainNotSupportUpstreamMTLS"
//  INVALIDPARAMETERVALUE_CERTIFICATEVERIFYCLIENTMUSTCA = "InvalidParameterValue.CertificateVerifyClientMustCa"
//  INVALIDPARAMETERVALUE_CERTIFICATEVERIFYCLIENTNEEDCERT = "InvalidParameterValue.CertificateVerifyClientNeedCert"
//  INVALIDPARAMETERVALUE_CERTIFICATEVERIFYUPSTREAMCLIENTMUSTRSAORECC = "InvalidParameterValue.CertificateVerifyUpstreamClientMustRSAorECC"
//  INVALIDPARAMETERVALUE_CERTIFICATEVERIFYUPSTREAMCLIENTMUSTSVR = "InvalidParameterValue.CertificateVerifyUpstreamClientMustSVR"
//  INVALIDPARAMETERVALUE_CERTIFICATEVERIFYUPSTREAMCLIENTNEEDCERT = "InvalidParameterValue.CertificateVerifyUpstreamClientNeedCert"
//  INVALIDPARAMETERVALUE_CLIENTCERTINFOQUOTALIMIT = "InvalidParameterValue.ClientCertInfoQuotaLimit"
//  INVALIDPARAMETERVALUE_INVALIDKEYLESSSERVERID = "InvalidParameterValue.InvalidKeylessServerId"
//  INVALIDPARAMETERVALUE_SERVERCERTINFONEEDCONTAINRSAORECC = "InvalidParameterValue.ServerCertInfoNeedContainRSAorECC"
//  INVALIDPARAMETERVALUE_SERVERCERTINFONEEDCONTAINSM2 = "InvalidParameterValue.ServerCertInfoNeedContainSM2"
//  INVALIDPARAMETERVALUE_UPSTREAMCLIENTCERTINFOQUOTALIMIT = "InvalidParameterValue.UpstreamClientCertInfoQuotaLimit"
//  LIMITEXCEEDED_RATELIMITEXCEEDED = "LimitExceeded.RateLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CERTIFICATEPRIVATEKEYISEMPTY = "OperationDenied.CertificatePrivateKeyIsEmpty"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_HOSTSCLIENTCERTIFICATEINCONSISTENCY = "OperationDenied.HostsClientCertificateInconsistency"
//  OPERATIONDENIED_HOSTSKEYLESSSERVERINCONSISTENCY = "OperationDenied.HostsKeylessServerInconsistency"
//  OPERATIONDENIED_HOSTSUPSTREAMCERTIFICATEINCONSISTENCY = "OperationDenied.HostsUpstreamCertificateInconsistency"
//  OPERATIONDENIED_KEYLESSCERTSWITCHTOFREECERTCONFLICT = "OperationDenied.KeylessCertSwitchToFreeCertConflict"
//  OPERATIONDENIED_KEYLESSMODECERTIFICATEPRIVATEKEYNEEDEMPTY = "OperationDenied.KeylessModeCertificatePrivateKeyNeedEmpty"
//  OPERATIONDENIED_NOTINKEYLESSWHITELIST = "OperationDenied.NotInKeylessWhiteList"
//  OPERATIONDENIED_NOTINUPSTREAMMTLSWHITELIST = "OperationDenied.NotInUpstreamMTLSWhiteList"
//  OPERATIONDENIED_UNSUPPORTTOCLOSEUPSTREAMMTLS = "OperationDenied.UnSupportToCloseUpstreamMTLS"
//  OPERATIONDENIED_USEUPSTREAMMTLSNEEDOPENHTTPS = "OperationDenied.UseUpstreamMTLSNeedOpenHttps"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEUNAVAILABLE_CERTNOTFOUND = "ResourceUnavailable.CertNotFound"
//  RESOURCEUNAVAILABLE_HOSTNOTFOUND = "ResourceUnavailable.HostNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func ModifyHostsCertificate(c *Client, request *ModifyHostsCertificateRequest) (response *ModifyHostsCertificateResponse, err error) {
    return ModifyHostsCertificateWithContext(context.Background(), c, request)
}

// ModifyHostsCertificate
// 完成域名创建之后，您可以为域名配置自有证书，也可以使用 EdgeOne 为您提供的 [免费证书](https://cloud.tencent.com/document/product/1552/90437)。
//
// 如果您需要配置自有证书，请先将证书上传至 [SSL证书控制台](https://console.cloud.tencent.com/certoverview)，然后在本接口中传入对应的证书 ID。详情参考 [部署自有证书至 EdgeOne 域名
//
// ](https://cloud.tencent.com/document/product/1552/88874)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CERTIFICATEHASEXPIRED = "FailedOperation.CertificateHasExpired"
//  FAILEDOPERATION_CERTIFICATENOTFOUND = "FailedOperation.CertificateNotFound"
//  FAILEDOPERATION_EDGECLIENTCERTIFICATEHASEXPIRED = "FailedOperation.EdgeClientCertificateHasExpired"
//  FAILEDOPERATION_INVALIDZONESTATUS = "FailedOperation.InvalidZoneStatus"
//  FAILEDOPERATION_MODIFYFAILED = "FailedOperation.ModifyFailed"
//  FAILEDOPERATION_UPSTREAMCLIENTCERTIFICATEHASEXPIRED = "FailedOperation.UpstreamClientCertificateHasExpired"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_GETROLEERROR = "InternalError.GetRoleError"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INTERNALERROR_UNKNOWERROR = "InternalError.UnknowError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_ALIASDOMAINNOTSUPPORTKEYLESS = "InvalidParameter.AliasDomainNotSupportKeyless"
//  INVALIDPARAMETER_CERTNOTMATCHDOMAIN = "InvalidParameter.CertNotMatchDomain"
//  INVALIDPARAMETER_CERTTOEXPIRE = "InvalidParameter.CertToExpire"
//  INVALIDPARAMETER_CERTTOOSHORTKEYSIZE = "InvalidParameter.CertTooShortKeySize"
//  INVALIDPARAMETER_CERTIFICATECONFLICTWITHKEYLESSSERVER = "InvalidParameter.CertificateConflictWithKeylessServer"
//  INVALIDPARAMETER_CNAMEWILDHOSTNOTALLOWAPPLYCERTIFICATE = "InvalidParameter.CnameWildHostNotAllowApplyCertificate"
//  INVALIDPARAMETER_EDGECLIENTCERTCHECKERROR = "InvalidParameter.EdgeClientCertCheckError"
//  INVALIDPARAMETER_HOSTSTATUSNOTALLOWAPPLYCERTIFICATE = "InvalidParameter.HostStatusNotAllowApplyCertificate"
//  INVALIDPARAMETER_INVALIDCERTINFO = "InvalidParameter.InvalidCertInfo"
//  INVALIDPARAMETER_INVALIDHTTPSCERTINFO = "InvalidParameter.InvalidHttpsCertInfo"
//  INVALIDPARAMETER_INVALIDHTTPSTLSVERSION = "InvalidParameter.InvalidHttpsTlsVersion"
//  INVALIDPARAMETER_UPSTREAMCLIENTCERTCHECKERROR = "InvalidParameter.UpstreamClientCertCheckError"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETERVALUE_ALIASDOMAINNOTSUPPORTEDGEMTLS = "InvalidParameterValue.AliasDomainNotSupportEdgeMTLS"
//  INVALIDPARAMETERVALUE_ALIASDOMAINNOTSUPPORTUPSTREAMMTLS = "InvalidParameterValue.AliasDomainNotSupportUpstreamMTLS"
//  INVALIDPARAMETERVALUE_CERTIFICATEVERIFYCLIENTMUSTCA = "InvalidParameterValue.CertificateVerifyClientMustCa"
//  INVALIDPARAMETERVALUE_CERTIFICATEVERIFYCLIENTNEEDCERT = "InvalidParameterValue.CertificateVerifyClientNeedCert"
//  INVALIDPARAMETERVALUE_CERTIFICATEVERIFYUPSTREAMCLIENTMUSTRSAORECC = "InvalidParameterValue.CertificateVerifyUpstreamClientMustRSAorECC"
//  INVALIDPARAMETERVALUE_CERTIFICATEVERIFYUPSTREAMCLIENTMUSTSVR = "InvalidParameterValue.CertificateVerifyUpstreamClientMustSVR"
//  INVALIDPARAMETERVALUE_CERTIFICATEVERIFYUPSTREAMCLIENTNEEDCERT = "InvalidParameterValue.CertificateVerifyUpstreamClientNeedCert"
//  INVALIDPARAMETERVALUE_CLIENTCERTINFOQUOTALIMIT = "InvalidParameterValue.ClientCertInfoQuotaLimit"
//  INVALIDPARAMETERVALUE_INVALIDKEYLESSSERVERID = "InvalidParameterValue.InvalidKeylessServerId"
//  INVALIDPARAMETERVALUE_SERVERCERTINFONEEDCONTAINRSAORECC = "InvalidParameterValue.ServerCertInfoNeedContainRSAorECC"
//  INVALIDPARAMETERVALUE_SERVERCERTINFONEEDCONTAINSM2 = "InvalidParameterValue.ServerCertInfoNeedContainSM2"
//  INVALIDPARAMETERVALUE_UPSTREAMCLIENTCERTINFOQUOTALIMIT = "InvalidParameterValue.UpstreamClientCertInfoQuotaLimit"
//  LIMITEXCEEDED_RATELIMITEXCEEDED = "LimitExceeded.RateLimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CERTIFICATEPRIVATEKEYISEMPTY = "OperationDenied.CertificatePrivateKeyIsEmpty"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_HOSTSCLIENTCERTIFICATEINCONSISTENCY = "OperationDenied.HostsClientCertificateInconsistency"
//  OPERATIONDENIED_HOSTSKEYLESSSERVERINCONSISTENCY = "OperationDenied.HostsKeylessServerInconsistency"
//  OPERATIONDENIED_HOSTSUPSTREAMCERTIFICATEINCONSISTENCY = "OperationDenied.HostsUpstreamCertificateInconsistency"
//  OPERATIONDENIED_KEYLESSCERTSWITCHTOFREECERTCONFLICT = "OperationDenied.KeylessCertSwitchToFreeCertConflict"
//  OPERATIONDENIED_KEYLESSMODECERTIFICATEPRIVATEKEYNEEDEMPTY = "OperationDenied.KeylessModeCertificatePrivateKeyNeedEmpty"
//  OPERATIONDENIED_NOTINKEYLESSWHITELIST = "OperationDenied.NotInKeylessWhiteList"
//  OPERATIONDENIED_NOTINUPSTREAMMTLSWHITELIST = "OperationDenied.NotInUpstreamMTLSWhiteList"
//  OPERATIONDENIED_UNSUPPORTTOCLOSEUPSTREAMMTLS = "OperationDenied.UnSupportToCloseUpstreamMTLS"
//  OPERATIONDENIED_USEUPSTREAMMTLSNEEDOPENHTTPS = "OperationDenied.UseUpstreamMTLSNeedOpenHttps"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCEUNAVAILABLE_CERTNOTFOUND = "ResourceUnavailable.CertNotFound"
//  RESOURCEUNAVAILABLE_HOSTNOTFOUND = "ResourceUnavailable.HostNotFound"
//  RESOURCEUNAVAILABLE_ZONENOTFOUND = "ResourceUnavailable.ZoneNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func ModifyHostsCertificateWithContext(ctx context.Context, c *Client, request *ModifyHostsCertificateRequest) (response *ModifyHostsCertificateResponse, err error) {
    if request == nil {
        request = NewModifyHostsCertificateRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyHostsCertificate")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyHostsCertificate require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyHostsCertificateResponse()
    err = c.Send(request, response)
    return
}

func NewModifyL4ProxyRequest() (request *ModifyL4ProxyRequest) {
    request = &ModifyL4ProxyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyL4Proxy")
    
    
    return
}

func NewModifyL4ProxyResponse() (response *ModifyL4ProxyResponse) {
    response = &ModifyL4ProxyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyL4Proxy
// 用于修改四层代理实例的配置。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDDISABLE = "OperationDenied.AccelerateMainlandDisable"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4LACKOFRESOURCES = "OperationDenied.L4LackOfResources"
//  OPERATIONDENIED_L4PROXYINOFFLINESTATUS = "OperationDenied.L4ProxyInOfflineStatus"
//  OPERATIONDENIED_L4PROXYINPROCESSSTATUS = "OperationDenied.L4ProxyInProcessStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func ModifyL4Proxy(c *Client, request *ModifyL4ProxyRequest) (response *ModifyL4ProxyResponse, err error) {
    return ModifyL4ProxyWithContext(context.Background(), c, request)
}

// ModifyL4Proxy
// 用于修改四层代理实例的配置。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDDISABLE = "OperationDenied.AccelerateMainlandDisable"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4LACKOFRESOURCES = "OperationDenied.L4LackOfResources"
//  OPERATIONDENIED_L4PROXYINOFFLINESTATUS = "OperationDenied.L4ProxyInOfflineStatus"
//  OPERATIONDENIED_L4PROXYINPROCESSSTATUS = "OperationDenied.L4ProxyInProcessStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func ModifyL4ProxyWithContext(ctx context.Context, c *Client, request *ModifyL4ProxyRequest) (response *ModifyL4ProxyResponse, err error) {
    if request == nil {
        request = NewModifyL4ProxyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyL4Proxy")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyL4Proxy require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyL4ProxyResponse()
    err = c.Send(request, response)
    return
}

func NewModifyL4ProxyRulesRequest() (request *ModifyL4ProxyRulesRequest) {
    request = &ModifyL4ProxyRulesRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyL4ProxyRules")
    
    
    return
}

func NewModifyL4ProxyRulesResponse() (response *ModifyL4ProxyRulesResponse) {
    response = &ModifyL4ProxyRulesResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyL4ProxyRules
// 用于修改四层代理转发规则，支持单条或者批量修改。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyL4ProxyRules(c *Client, request *ModifyL4ProxyRulesRequest) (response *ModifyL4ProxyRulesResponse, err error) {
    return ModifyL4ProxyRulesWithContext(context.Background(), c, request)
}

// ModifyL4ProxyRules
// 用于修改四层代理转发规则，支持单条或者批量修改。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_PARAMETERERROR = "InvalidParameter.ParameterError"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyL4ProxyRulesWithContext(ctx context.Context, c *Client, request *ModifyL4ProxyRulesRequest) (response *ModifyL4ProxyRulesResponse, err error) {
    if request == nil {
        request = NewModifyL4ProxyRulesRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyL4ProxyRules")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyL4ProxyRules require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyL4ProxyRulesResponse()
    err = c.Send(request, response)
    return
}

func NewModifyL4ProxyRulesStatusRequest() (request *ModifyL4ProxyRulesStatusRequest) {
    request = &ModifyL4ProxyRulesStatusRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyL4ProxyRulesStatus")
    
    
    return
}

func NewModifyL4ProxyRulesStatusResponse() (response *ModifyL4ProxyRulesStatusResponse) {
    response = &ModifyL4ProxyRulesStatusResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyL4ProxyRulesStatus
// 用于启用/停用四层代理转发规则状态，支持单条或者批量操作。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyL4ProxyRulesStatus(c *Client, request *ModifyL4ProxyRulesStatusRequest) (response *ModifyL4ProxyRulesStatusResponse, err error) {
    return ModifyL4ProxyRulesStatusWithContext(context.Background(), c, request)
}

// ModifyL4ProxyRulesStatus
// 用于启用/停用四层代理转发规则状态，支持单条或者批量操作。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyL4ProxyRulesStatusWithContext(ctx context.Context, c *Client, request *ModifyL4ProxyRulesStatusRequest) (response *ModifyL4ProxyRulesStatusResponse, err error) {
    if request == nil {
        request = NewModifyL4ProxyRulesStatusRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyL4ProxyRulesStatus")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyL4ProxyRulesStatus require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyL4ProxyRulesStatusResponse()
    err = c.Send(request, response)
    return
}

func NewModifyL4ProxyStatusRequest() (request *ModifyL4ProxyStatusRequest) {
    request = &ModifyL4ProxyStatusRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyL4ProxyStatus")
    
    
    return
}

func NewModifyL4ProxyStatusResponse() (response *ModifyL4ProxyStatusResponse) {
    response = &ModifyL4ProxyStatusResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyL4ProxyStatus
// 用于启用/停用四层代理实例。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyL4ProxyStatus(c *Client, request *ModifyL4ProxyStatusRequest) (response *ModifyL4ProxyStatusResponse, err error) {
    return ModifyL4ProxyStatusWithContext(context.Background(), c, request)
}

// ModifyL4ProxyStatus
// 用于启用/停用四层代理实例。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_L4PROXYINBANNEDSTATUS = "OperationDenied.L4ProxyInBannedStatus"
//  RESOURCENOTFOUND = "ResourceNotFound"
func ModifyL4ProxyStatusWithContext(ctx context.Context, c *Client, request *ModifyL4ProxyStatusRequest) (response *ModifyL4ProxyStatusResponse, err error) {
    if request == nil {
        request = NewModifyL4ProxyStatusRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyL4ProxyStatus")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyL4ProxyStatus require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyL4ProxyStatusResponse()
    err = c.Send(request, response)
    return
}

func NewModifyL7AccRuleRequest() (request *ModifyL7AccRuleRequest) {
    request = &ModifyL7AccRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyL7AccRule")
    
    
    return
}

func NewModifyL7AccRuleResponse() (response *ModifyL7AccRuleResponse) {
    response = &ModifyL7AccRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyL7AccRule
// 本接口用于修改[规则引擎](https://cloud.tencent.com/document/product/1552/70901)中的规则，单次仅支持修改单条规则。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_COMPRESSIONINVALIDALGORITHMS = "InvalidParameter.CompressionInvalidAlgorithms"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONDUPLICATEACTION = "InvalidParameter.ErrInvalidActionDuplicateAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMTOOMANYVALUES = "InvalidParameter.ErrInvalidActionParamTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONHOSTTOOMANYWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionHostTooManyWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDELSEWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidElseWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESECRETKEY = "InvalidParameter.InvalidAuthenticationTypeSecretKey"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEFORMAT = "InvalidParameter.InvalidAuthenticationTypeTimeFormat"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEPARAM = "InvalidParameter.InvalidAuthenticationTypeTimeParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYCOOKIE = "InvalidParameter.InvalidCacheKeyCookie"
//  INVALIDPARAMETER_INVALIDCACHEKEYIGNORECASE = "InvalidParameter.InvalidCacheKeyIgnoreCase"
//  INVALIDPARAMETER_INVALIDCACHEKEYSCHEME = "InvalidParameter.InvalidCacheKeyScheme"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDCOSDOMAIN = "InvalidParameter.InvalidCosDomain"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_ORIGINPULLPROTOCOLISREQUIRED = "InvalidParameter.OriginPullProtocolIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_RESPONSEHEADERCACHECONTROLNOTALLOWDELETE = "InvalidParameter.ResponseHeaderCacheControlNotAllowDelete"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETER_TLSVERSIONNOTINSEQUENCE = "InvalidParameter.TlsVersionNotInSequence"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_TRIALPLANRESPONSEPAGE = "InvalidParameterValue.TrialPlanResponsePage"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifyL7AccRule(c *Client, request *ModifyL7AccRuleRequest) (response *ModifyL7AccRuleResponse, err error) {
    return ModifyL7AccRuleWithContext(context.Background(), c, request)
}

// ModifyL7AccRule
// 本接口用于修改[规则引擎](https://cloud.tencent.com/document/product/1552/70901)中的规则，单次仅支持修改单条规则。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_COMPRESSIONINVALIDALGORITHMS = "InvalidParameter.CompressionInvalidAlgorithms"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONDUPLICATEACTION = "InvalidParameter.ErrInvalidActionDuplicateAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMTOOMANYVALUES = "InvalidParameter.ErrInvalidActionParamTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONHOSTTOOMANYWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionHostTooManyWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDELSEWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidElseWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESECRETKEY = "InvalidParameter.InvalidAuthenticationTypeSecretKey"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEFORMAT = "InvalidParameter.InvalidAuthenticationTypeTimeFormat"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEPARAM = "InvalidParameter.InvalidAuthenticationTypeTimeParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYCOOKIE = "InvalidParameter.InvalidCacheKeyCookie"
//  INVALIDPARAMETER_INVALIDCACHEKEYIGNORECASE = "InvalidParameter.InvalidCacheKeyIgnoreCase"
//  INVALIDPARAMETER_INVALIDCACHEKEYSCHEME = "InvalidParameter.InvalidCacheKeyScheme"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDCOSDOMAIN = "InvalidParameter.InvalidCosDomain"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_ORIGINPULLPROTOCOLISREQUIRED = "InvalidParameter.OriginPullProtocolIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_RESPONSEHEADERCACHECONTROLNOTALLOWDELETE = "InvalidParameter.ResponseHeaderCacheControlNotAllowDelete"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETER_TLSVERSIONNOTINSEQUENCE = "InvalidParameter.TlsVersionNotInSequence"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_TRIALPLANRESPONSEPAGE = "InvalidParameterValue.TrialPlanResponsePage"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifyL7AccRuleWithContext(ctx context.Context, c *Client, request *ModifyL7AccRuleRequest) (response *ModifyL7AccRuleResponse, err error) {
    if request == nil {
        request = NewModifyL7AccRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyL7AccRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyL7AccRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyL7AccRuleResponse()
    err = c.Send(request, response)
    return
}

func NewModifyL7AccRulePriorityRequest() (request *ModifyL7AccRulePriorityRequest) {
    request = &ModifyL7AccRulePriorityRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyL7AccRulePriority")
    
    
    return
}

func NewModifyL7AccRulePriorityResponse() (response *ModifyL7AccRulePriorityResponse) {
    response = &ModifyL7AccRulePriorityResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyL7AccRulePriority
// 本接口用于修改[规则引擎](https://cloud.tencent.com/document/product/1552/70901)中规则列表的优先级，本接口需要传入站点 ID 下完整的规则 ID 列表，规则 ID 列表可以通过[查询七层加速规则](https://cloud.tencent.com/document/product/1552/115820)接口获取，最终优先级顺序将调整成规则 ID 列表的顺序，从前往后执行。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_INVALIDRULEENGINE = "InvalidParameter.InvalidRuleEngine"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
func ModifyL7AccRulePriority(c *Client, request *ModifyL7AccRulePriorityRequest) (response *ModifyL7AccRulePriorityResponse, err error) {
    return ModifyL7AccRulePriorityWithContext(context.Background(), c, request)
}

// ModifyL7AccRulePriority
// 本接口用于修改[规则引擎](https://cloud.tencent.com/document/product/1552/70901)中规则列表的优先级，本接口需要传入站点 ID 下完整的规则 ID 列表，规则 ID 列表可以通过[查询七层加速规则](https://cloud.tencent.com/document/product/1552/115820)接口获取，最终优先级顺序将调整成规则 ID 列表的顺序，从前往后执行。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_INVALIDRULEENGINE = "InvalidParameter.InvalidRuleEngine"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
func ModifyL7AccRulePriorityWithContext(ctx context.Context, c *Client, request *ModifyL7AccRulePriorityRequest) (response *ModifyL7AccRulePriorityResponse, err error) {
    if request == nil {
        request = NewModifyL7AccRulePriorityRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyL7AccRulePriority")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyL7AccRulePriority require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyL7AccRulePriorityResponse()
    err = c.Send(request, response)
    return
}

func NewModifyL7AccSettingRequest() (request *ModifyL7AccSettingRequest) {
    request = &ModifyL7AccSettingRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyL7AccSetting")
    
    
    return
}

func NewModifyL7AccSettingResponse() (response *ModifyL7AccSettingResponse) {
    response = &ModifyL7AccSettingResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyL7AccSetting
// 本接口用于修改[站点加速](https://cloud.tencent.com/document/product/1552/96193)全局配置。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CERTIFICATENOTFOUND = "FailedOperation.CertificateNotFound"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INTERNALERROR_UNKNOWERROR = "InternalError.UnknowError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGREQUIRESFULLURLCACHEOFF = "InvalidParameter.CacheKeyQueryStringRequiresFullUrlCacheOff"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_CLIENTIPCOUNTRYCONFLICTSWITHIPV6 = "InvalidParameter.ClientIpCountryConflictsWithIpv6"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_INVALIDAWSPRIVATEACCESS = "InvalidParameter.InvalidAwsPrivateAccess"
//  INVALIDPARAMETER_INVALIDCACHECONFIGFOLLOWORIGIN = "InvalidParameter.InvalidCacheConfigFollowOrigin"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHEONLYONSWITCH = "InvalidParameter.InvalidCacheOnlyOnSwitch"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPCOUNTRYHEADERNAME = "InvalidParameter.InvalidClientIpCountryHeaderName"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDFORCEREDIRECTTYPE = "InvalidParameter.InvalidForceRedirectType"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCERTINFO = "InvalidParameter.InvalidHttpsCertInfo"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDHTTPSTLSVERSION = "InvalidParameter.InvalidHttpsTlsVersion"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDORIGINTYPE = "InvalidParameter.InvalidOriginType"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTMAXSIZEBILLING = "InvalidParameter.InvalidPostMaxSizeBilling"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDRESOURCEIDBILLING = "InvalidParameter.InvalidResourceIdBilling"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGCLIENTIP = "InvalidParameter.InvalidStandardDebugClientIp"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEMPTYLIST = "InvalidParameter.InvalidStandardDebugEmptyList"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_MULTIPLYLAYERNOTSUPPORTSMARTROUTING = "InvalidParameter.MultiplyLayerNotSupportSmartRouting"
//  INVALIDPARAMETER_OCDIRECTORIGINREQUIRESSMARTROUTING = "InvalidParameter.OCDirectOriginRequiresSmartRouting"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_SETTINGINVALIDPARAM = "InvalidParameter.SettingInvalidParam"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDDISABLE = "OperationDenied.AccelerateMainlandDisable"
//  OPERATIONDENIED_ACCELERATEMAINLANDIPV6CONFLICT = "OperationDenied.AccelerateMainlandIpv6Conflict"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_SHAREDCNAMEUNSUPPORTEDACCELERATEMAINLAND = "OperationDenied.SharedCNAMEUnsupportedAccelerateMainland"
//  OPERATIONDENIED_SHAREDCNAMEUNSUPPORTEDIPV6 = "OperationDenied.SharedCNAMEUnsupportedIPv6"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCENOTFOUND_POSTMAXSIZEQUOTANOTFOUND = "ResourceNotFound.PostMaxSizeQuotaNotFound"
//  RESOURCEUNAVAILABLE_CERTNOTFOUND = "ResourceUnavailable.CertNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func ModifyL7AccSetting(c *Client, request *ModifyL7AccSettingRequest) (response *ModifyL7AccSettingResponse, err error) {
    return ModifyL7AccSettingWithContext(context.Background(), c, request)
}

// ModifyL7AccSetting
// 本接口用于修改[站点加速](https://cloud.tencent.com/document/product/1552/96193)全局配置。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CERTIFICATENOTFOUND = "FailedOperation.CertificateNotFound"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INTERNALERROR_UNKNOWERROR = "InternalError.UnknowError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGREQUIRESFULLURLCACHEOFF = "InvalidParameter.CacheKeyQueryStringRequiresFullUrlCacheOff"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_CLIENTIPCOUNTRYCONFLICTSWITHIPV6 = "InvalidParameter.ClientIpCountryConflictsWithIpv6"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_INVALIDAWSPRIVATEACCESS = "InvalidParameter.InvalidAwsPrivateAccess"
//  INVALIDPARAMETER_INVALIDCACHECONFIGFOLLOWORIGIN = "InvalidParameter.InvalidCacheConfigFollowOrigin"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHEONLYONSWITCH = "InvalidParameter.InvalidCacheOnlyOnSwitch"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPCOUNTRYHEADERNAME = "InvalidParameter.InvalidClientIpCountryHeaderName"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDFORCEREDIRECTTYPE = "InvalidParameter.InvalidForceRedirectType"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCERTINFO = "InvalidParameter.InvalidHttpsCertInfo"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDHTTPSTLSVERSION = "InvalidParameter.InvalidHttpsTlsVersion"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDORIGINTYPE = "InvalidParameter.InvalidOriginType"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTMAXSIZEBILLING = "InvalidParameter.InvalidPostMaxSizeBilling"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDRESOURCEIDBILLING = "InvalidParameter.InvalidResourceIdBilling"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGCLIENTIP = "InvalidParameter.InvalidStandardDebugClientIp"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEMPTYLIST = "InvalidParameter.InvalidStandardDebugEmptyList"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_MULTIPLYLAYERNOTSUPPORTSMARTROUTING = "InvalidParameter.MultiplyLayerNotSupportSmartRouting"
//  INVALIDPARAMETER_OCDIRECTORIGINREQUIRESSMARTROUTING = "InvalidParameter.OCDirectOriginRequiresSmartRouting"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_SETTINGINVALIDPARAM = "InvalidParameter.SettingInvalidParam"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDDISABLE = "OperationDenied.AccelerateMainlandDisable"
//  OPERATIONDENIED_ACCELERATEMAINLANDIPV6CONFLICT = "OperationDenied.AccelerateMainlandIpv6Conflict"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_SHAREDCNAMEUNSUPPORTEDACCELERATEMAINLAND = "OperationDenied.SharedCNAMEUnsupportedAccelerateMainland"
//  OPERATIONDENIED_SHAREDCNAMEUNSUPPORTEDIPV6 = "OperationDenied.SharedCNAMEUnsupportedIPv6"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCENOTFOUND_POSTMAXSIZEQUOTANOTFOUND = "ResourceNotFound.PostMaxSizeQuotaNotFound"
//  RESOURCEUNAVAILABLE_CERTNOTFOUND = "ResourceUnavailable.CertNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func ModifyL7AccSettingWithContext(ctx context.Context, c *Client, request *ModifyL7AccSettingRequest) (response *ModifyL7AccSettingResponse, err error) {
    if request == nil {
        request = NewModifyL7AccSettingRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyL7AccSetting")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyL7AccSetting require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyL7AccSettingResponse()
    err = c.Send(request, response)
    return
}

func NewModifyLoadBalancerRequest() (request *ModifyLoadBalancerRequest) {
    request = &ModifyLoadBalancerRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyLoadBalancer")
    
    
    return
}

func NewModifyLoadBalancerResponse() (response *ModifyLoadBalancerResponse) {
    response = &ModifyLoadBalancerResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyLoadBalancer
// 修改负载均衡实例配置。负载均衡功能内测中，如您需要使用请 [联系我们](https://cloud.tencent.com/online-service)。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_LOADBALANCERBINDL4NOTINSTABLESTATUS = "InvalidParameter.LoadBalancerBindL4NotInStableStatus"
//  INVALIDPARAMETER_LOADBALANCERBINDL7NOTINSTABLESTATUS = "InvalidParameter.LoadBalancerBindL7NotInStableStatus"
func ModifyLoadBalancer(c *Client, request *ModifyLoadBalancerRequest) (response *ModifyLoadBalancerResponse, err error) {
    return ModifyLoadBalancerWithContext(context.Background(), c, request)
}

// ModifyLoadBalancer
// 修改负载均衡实例配置。负载均衡功能内测中，如您需要使用请 [联系我们](https://cloud.tencent.com/online-service)。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_LOADBALANCERBINDL4NOTINSTABLESTATUS = "InvalidParameter.LoadBalancerBindL4NotInStableStatus"
//  INVALIDPARAMETER_LOADBALANCERBINDL7NOTINSTABLESTATUS = "InvalidParameter.LoadBalancerBindL7NotInStableStatus"
func ModifyLoadBalancerWithContext(ctx context.Context, c *Client, request *ModifyLoadBalancerRequest) (response *ModifyLoadBalancerResponse, err error) {
    if request == nil {
        request = NewModifyLoadBalancerRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyLoadBalancer")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyLoadBalancer require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyLoadBalancerResponse()
    err = c.Send(request, response)
    return
}

func NewModifyMultiPathGatewayRequest() (request *ModifyMultiPathGatewayRequest) {
    request = &ModifyMultiPathGatewayRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyMultiPathGateway")
    
    
    return
}

func NewModifyMultiPathGatewayResponse() (response *ModifyMultiPathGatewayResponse) {
    response = &ModifyMultiPathGatewayResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyMultiPathGateway
// 通过本接口修改多通道安全加速网关信息，如名称、网关 ID、IP、端口等。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func ModifyMultiPathGateway(c *Client, request *ModifyMultiPathGatewayRequest) (response *ModifyMultiPathGatewayResponse, err error) {
    return ModifyMultiPathGatewayWithContext(context.Background(), c, request)
}

// ModifyMultiPathGateway
// 通过本接口修改多通道安全加速网关信息，如名称、网关 ID、IP、端口等。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func ModifyMultiPathGatewayWithContext(ctx context.Context, c *Client, request *ModifyMultiPathGatewayRequest) (response *ModifyMultiPathGatewayResponse, err error) {
    if request == nil {
        request = NewModifyMultiPathGatewayRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyMultiPathGateway")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyMultiPathGateway require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyMultiPathGatewayResponse()
    err = c.Send(request, response)
    return
}

func NewModifyMultiPathGatewayLineRequest() (request *ModifyMultiPathGatewayLineRequest) {
    request = &ModifyMultiPathGatewayLineRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyMultiPathGatewayLine")
    
    
    return
}

func NewModifyMultiPathGatewayLineResponse() (response *ModifyMultiPathGatewayLineResponse) {
    response = &ModifyMultiPathGatewayLineResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyMultiPathGatewayLine
// 通过本接口修改接入多通道安全加速网关的线路，包括 EdgeOne 四层代理线路、自定义线路。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func ModifyMultiPathGatewayLine(c *Client, request *ModifyMultiPathGatewayLineRequest) (response *ModifyMultiPathGatewayLineResponse, err error) {
    return ModifyMultiPathGatewayLineWithContext(context.Background(), c, request)
}

// ModifyMultiPathGatewayLine
// 通过本接口修改接入多通道安全加速网关的线路，包括 EdgeOne 四层代理线路、自定义线路。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func ModifyMultiPathGatewayLineWithContext(ctx context.Context, c *Client, request *ModifyMultiPathGatewayLineRequest) (response *ModifyMultiPathGatewayLineResponse, err error) {
    if request == nil {
        request = NewModifyMultiPathGatewayLineRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyMultiPathGatewayLine")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyMultiPathGatewayLine require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyMultiPathGatewayLineResponse()
    err = c.Send(request, response)
    return
}

func NewModifyMultiPathGatewaySecretKeyRequest() (request *ModifyMultiPathGatewaySecretKeyRequest) {
    request = &ModifyMultiPathGatewaySecretKeyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyMultiPathGatewaySecretKey")
    
    
    return
}

func NewModifyMultiPathGatewaySecretKeyResponse() (response *ModifyMultiPathGatewaySecretKeyResponse) {
    response = &ModifyMultiPathGatewaySecretKeyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyMultiPathGatewaySecretKey
// 通过本接口修改接入多通道安全加速网关的密钥，客户基于接入密钥签名接入多通道安全加速网关，修改后原密钥失效。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func ModifyMultiPathGatewaySecretKey(c *Client, request *ModifyMultiPathGatewaySecretKeyRequest) (response *ModifyMultiPathGatewaySecretKeyResponse, err error) {
    return ModifyMultiPathGatewaySecretKeyWithContext(context.Background(), c, request)
}

// ModifyMultiPathGatewaySecretKey
// 通过本接口修改接入多通道安全加速网关的密钥，客户基于接入密钥签名接入多通道安全加速网关，修改后原密钥失效。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func ModifyMultiPathGatewaySecretKeyWithContext(ctx context.Context, c *Client, request *ModifyMultiPathGatewaySecretKeyRequest) (response *ModifyMultiPathGatewaySecretKeyResponse, err error) {
    if request == nil {
        request = NewModifyMultiPathGatewaySecretKeyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyMultiPathGatewaySecretKey")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyMultiPathGatewaySecretKey require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyMultiPathGatewaySecretKeyResponse()
    err = c.Send(request, response)
    return
}

func NewModifyMultiPathGatewayStatusRequest() (request *ModifyMultiPathGatewayStatusRequest) {
    request = &ModifyMultiPathGatewayStatusRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyMultiPathGatewayStatus")
    
    
    return
}

func NewModifyMultiPathGatewayStatusResponse() (response *ModifyMultiPathGatewayStatusResponse) {
    response = &ModifyMultiPathGatewayStatusResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyMultiPathGatewayStatus
// 更新多通道安全网关状态。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func ModifyMultiPathGatewayStatus(c *Client, request *ModifyMultiPathGatewayStatusRequest) (response *ModifyMultiPathGatewayStatusResponse, err error) {
    return ModifyMultiPathGatewayStatusWithContext(context.Background(), c, request)
}

// ModifyMultiPathGatewayStatus
// 更新多通道安全网关状态。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func ModifyMultiPathGatewayStatusWithContext(ctx context.Context, c *Client, request *ModifyMultiPathGatewayStatusRequest) (response *ModifyMultiPathGatewayStatusResponse, err error) {
    if request == nil {
        request = NewModifyMultiPathGatewayStatusRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyMultiPathGatewayStatus")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyMultiPathGatewayStatus require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyMultiPathGatewayStatusResponse()
    err = c.Send(request, response)
    return
}

func NewModifyOriginACLRequest() (request *ModifyOriginACLRequest) {
    request = &ModifyOriginACLRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyOriginACL")
    
    
    return
}

func NewModifyOriginACLResponse() (response *ModifyOriginACLResponse) {
    response = &ModifyOriginACLResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyOriginACL
// 本接口用于对七层加速域名/四层代理实例启用/关闭特定回源 IP 网段回源。单次支持提交的七层加速域名的数量最大为 200，四层代理实例的数量最大为 100，支持七层加速域名/四层代理实例混合提交，总实例个数最大为 200。如需变更超过 200 个实例，请通过本接口分批提交。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_INVALIDDOMAINS = "InvalidParameter.InvalidDomains"
//  INVALIDPARAMETER_INVALIDPROXIES = "InvalidParameter.InvalidProxies"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_UNSUPPORTEDPLAN = "OperationDenied.UnsupportedPlan"
//  OPERATIONDENIED_UPDATEIPWHITELISTFIRST = "OperationDenied.UpdateIPWhitelistFirst"
func ModifyOriginACL(c *Client, request *ModifyOriginACLRequest) (response *ModifyOriginACLResponse, err error) {
    return ModifyOriginACLWithContext(context.Background(), c, request)
}

// ModifyOriginACL
// 本接口用于对七层加速域名/四层代理实例启用/关闭特定回源 IP 网段回源。单次支持提交的七层加速域名的数量最大为 200，四层代理实例的数量最大为 100，支持七层加速域名/四层代理实例混合提交，总实例个数最大为 200。如需变更超过 200 个实例，请通过本接口分批提交。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETER_INVALIDDOMAINS = "InvalidParameter.InvalidDomains"
//  INVALIDPARAMETER_INVALIDPROXIES = "InvalidParameter.InvalidProxies"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_UNSUPPORTEDPLAN = "OperationDenied.UnsupportedPlan"
//  OPERATIONDENIED_UPDATEIPWHITELISTFIRST = "OperationDenied.UpdateIPWhitelistFirst"
func ModifyOriginACLWithContext(ctx context.Context, c *Client, request *ModifyOriginACLRequest) (response *ModifyOriginACLResponse, err error) {
    if request == nil {
        request = NewModifyOriginACLRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyOriginACL")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyOriginACL require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyOriginACLResponse()
    err = c.Send(request, response)
    return
}

func NewModifyOriginGroupRequest() (request *ModifyOriginGroupRequest) {
    request = &ModifyOriginGroupRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyOriginGroup")
    
    
    return
}

func NewModifyOriginGroupResponse() (response *ModifyOriginGroupResponse) {
    response = &ModifyOriginGroupResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyOriginGroup
// 修改源站组配置，新提交的源站记录将会覆盖原有源站组中的源站记录。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_HOSTHEADERINVALID = "InvalidParameter.HostHeaderInvalid"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_ORIGINL4RECORDIPV4MIXDOMAIN = "InvalidParameter.OriginL4RecordIPV4MixDomain"
//  INVALIDPARAMETER_ORIGINL4RECORDMULTIDOMAIN = "InvalidParameter.OriginL4RecordMultiDomain"
//  INVALIDPARAMETER_ORIGINNAMEEXISTS = "InvalidParameter.OriginNameExists"
//  INVALIDPARAMETER_ORIGINRECORDFORMATERROR = "InvalidParameter.OriginRecordFormatError"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATIONDOMAINSTATUSNOTINONLINE = "OperationDenied.AccelerationDomainStatusNotInOnline"
//  OPERATIONDENIED_L4STATUSNOTINONLINE = "OperationDenied.L4StatusNotInOnline"
//  OPERATIONDENIED_LOADBALANCESTATUSNOTINONLINE = "OperationDenied.LoadBalanceStatusNotInOnline"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifyOriginGroup(c *Client, request *ModifyOriginGroupRequest) (response *ModifyOriginGroupResponse, err error) {
    return ModifyOriginGroupWithContext(context.Background(), c, request)
}

// ModifyOriginGroup
// 修改源站组配置，新提交的源站记录将会覆盖原有源站组中的源站记录。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  INVALIDPARAMETER_HOSTHEADERINVALID = "InvalidParameter.HostHeaderInvalid"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_ORIGINL4RECORDIPV4MIXDOMAIN = "InvalidParameter.OriginL4RecordIPV4MixDomain"
//  INVALIDPARAMETER_ORIGINL4RECORDMULTIDOMAIN = "InvalidParameter.OriginL4RecordMultiDomain"
//  INVALIDPARAMETER_ORIGINNAMEEXISTS = "InvalidParameter.OriginNameExists"
//  INVALIDPARAMETER_ORIGINRECORDFORMATERROR = "InvalidParameter.OriginRecordFormatError"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATIONDOMAINSTATUSNOTINONLINE = "OperationDenied.AccelerationDomainStatusNotInOnline"
//  OPERATIONDENIED_L4STATUSNOTINONLINE = "OperationDenied.L4StatusNotInOnline"
//  OPERATIONDENIED_LOADBALANCESTATUSNOTINONLINE = "OperationDenied.LoadBalanceStatusNotInOnline"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifyOriginGroupWithContext(ctx context.Context, c *Client, request *ModifyOriginGroupRequest) (response *ModifyOriginGroupResponse, err error) {
    if request == nil {
        request = NewModifyOriginGroupRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyOriginGroup")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyOriginGroup require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyOriginGroupResponse()
    err = c.Send(request, response)
    return
}

func NewModifyPlanRequest() (request *ModifyPlanRequest) {
    request = &ModifyPlanRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyPlan")
    
    
    return
}

func NewModifyPlanResponse() (response *ModifyPlanResponse) {
    response = &ModifyPlanResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyPlan
// 修改套餐配置。目前仅支持修改预付费套餐的自动续费开关。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  OPERATIONDENIED_ENTERPRISEPLANAUTORENEWUNSUPPORTED = "OperationDenied.EnterprisePlanAutoRenewUnsupported"
//  OPERATIONDENIED_PLANHASBEENISOLATED = "OperationDenied.PlanHasBeenIsolated"
func ModifyPlan(c *Client, request *ModifyPlanRequest) (response *ModifyPlanResponse, err error) {
    return ModifyPlanWithContext(context.Background(), c, request)
}

// ModifyPlan
// 修改套餐配置。目前仅支持修改预付费套餐的自动续费开关。
//
// 可能返回的错误码:
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  OPERATIONDENIED_ENTERPRISEPLANAUTORENEWUNSUPPORTED = "OperationDenied.EnterprisePlanAutoRenewUnsupported"
//  OPERATIONDENIED_PLANHASBEENISOLATED = "OperationDenied.PlanHasBeenIsolated"
func ModifyPlanWithContext(ctx context.Context, c *Client, request *ModifyPlanRequest) (response *ModifyPlanResponse, err error) {
    if request == nil {
        request = NewModifyPlanRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyPlan")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyPlan require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyPlanResponse()
    err = c.Send(request, response)
    return
}

func NewModifyRealtimeLogDeliveryTaskRequest() (request *ModifyRealtimeLogDeliveryTaskRequest) {
    request = &ModifyRealtimeLogDeliveryTaskRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyRealtimeLogDeliveryTask")
    
    
    return
}

func NewModifyRealtimeLogDeliveryTaskResponse() (response *ModifyRealtimeLogDeliveryTaskResponse) {
    response = &ModifyRealtimeLogDeliveryTaskResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyRealtimeLogDeliveryTask
// 通过本接口修改实时日志投递任务配置。本接口有如下限制：<li>不支持修改实时日志投递任务目的地类型（TaskType）；</li><li>不支持修改数据投递类型（LogType）</li><li>不支持修改数据投递区域（Area）</li><li>当原实时日志投递任务的目的地为腾讯云 CLS 时，不支持修改目的地详细配置，如日志集、日志主题。</li>
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CREATELOGTOPICTASKAUTHFAILURE = "FailedOperation.CreateLogTopicTaskAuthFailure"
//  INVALIDPARAMETER_INVALIDLOGFORMATFIELDDELIMITER = "InvalidParameter.InvalidLogFormatFieldDelimiter"
//  INVALIDPARAMETER_INVALIDLOGFORMATFORMATTYPE = "InvalidParameter.InvalidLogFormatFormatType"
//  INVALIDPARAMETER_INVALIDLOGFORMATRECORDDELIMITER = "InvalidParameter.InvalidLogFormatRecordDelimiter"
//  LIMITEXCEEDED_CUSTOMLOGFIELDREGEXLIMITEXCEEDED = "LimitExceeded.CustomLogFieldRegexLimitExceeded"
func ModifyRealtimeLogDeliveryTask(c *Client, request *ModifyRealtimeLogDeliveryTaskRequest) (response *ModifyRealtimeLogDeliveryTaskResponse, err error) {
    return ModifyRealtimeLogDeliveryTaskWithContext(context.Background(), c, request)
}

// ModifyRealtimeLogDeliveryTask
// 通过本接口修改实时日志投递任务配置。本接口有如下限制：<li>不支持修改实时日志投递任务目的地类型（TaskType）；</li><li>不支持修改数据投递类型（LogType）</li><li>不支持修改数据投递区域（Area）</li><li>当原实时日志投递任务的目的地为腾讯云 CLS 时，不支持修改目的地详细配置，如日志集、日志主题。</li>
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CREATELOGTOPICTASKAUTHFAILURE = "FailedOperation.CreateLogTopicTaskAuthFailure"
//  INVALIDPARAMETER_INVALIDLOGFORMATFIELDDELIMITER = "InvalidParameter.InvalidLogFormatFieldDelimiter"
//  INVALIDPARAMETER_INVALIDLOGFORMATFORMATTYPE = "InvalidParameter.InvalidLogFormatFormatType"
//  INVALIDPARAMETER_INVALIDLOGFORMATRECORDDELIMITER = "InvalidParameter.InvalidLogFormatRecordDelimiter"
//  LIMITEXCEEDED_CUSTOMLOGFIELDREGEXLIMITEXCEEDED = "LimitExceeded.CustomLogFieldRegexLimitExceeded"
func ModifyRealtimeLogDeliveryTaskWithContext(ctx context.Context, c *Client, request *ModifyRealtimeLogDeliveryTaskRequest) (response *ModifyRealtimeLogDeliveryTaskResponse, err error) {
    if request == nil {
        request = NewModifyRealtimeLogDeliveryTaskRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyRealtimeLogDeliveryTask")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyRealtimeLogDeliveryTask require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyRealtimeLogDeliveryTaskResponse()
    err = c.Send(request, response)
    return
}

func NewModifyRuleRequest() (request *ModifyRuleRequest) {
    request = &ModifyRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyRule")
    
    
    return
}

func NewModifyRuleResponse() (response *ModifyRuleResponse) {
    response = &ModifyRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyRule
// 本接口为旧版本修改规则引擎接口，EdgeOne 于 2025 年 1 月 21 日已对规则引擎相关接口全面升级，新版本修改七层加速规则接口详情请参考 [ModifyL7AccRule](https://cloud.tencent.com/document/product/1552/115818)。
//
// <p style="color: red;">注意：自 2025 年 1 月 21 日起，旧版接口停止更新迭代，后续新增功能将仅在新版接口中提供，旧版接口支持的原有能力将不受影响。为避免在使用旧版接口时出现数据字段冲突，建议您尽早迁移到新版规则引擎接口。</p>
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_COMPRESSIONINVALIDALGORITHMS = "InvalidParameter.CompressionInvalidAlgorithms"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONDUPLICATEACTION = "InvalidParameter.ErrInvalidActionDuplicateAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMTOOMANYVALUES = "InvalidParameter.ErrInvalidActionParamTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONHOSTTOOMANYWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionHostTooManyWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDELSEWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidElseWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESECRETKEY = "InvalidParameter.InvalidAuthenticationTypeSecretKey"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEFORMAT = "InvalidParameter.InvalidAuthenticationTypeTimeFormat"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEPARAM = "InvalidParameter.InvalidAuthenticationTypeTimeParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYCOOKIE = "InvalidParameter.InvalidCacheKeyCookie"
//  INVALIDPARAMETER_INVALIDCACHEKEYIGNORECASE = "InvalidParameter.InvalidCacheKeyIgnoreCase"
//  INVALIDPARAMETER_INVALIDCACHEKEYSCHEME = "InvalidParameter.InvalidCacheKeyScheme"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDCOSDOMAIN = "InvalidParameter.InvalidCosDomain"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_ORIGINPULLPROTOCOLISREQUIRED = "InvalidParameter.OriginPullProtocolIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_RESPONSEHEADERCACHECONTROLNOTALLOWDELETE = "InvalidParameter.ResponseHeaderCacheControlNotAllowDelete"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETER_TLSVERSIONNOTINSEQUENCE = "InvalidParameter.TlsVersionNotInSequence"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_TRIALPLANRESPONSEPAGE = "InvalidParameterValue.TrialPlanResponsePage"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifyRule(c *Client, request *ModifyRuleRequest) (response *ModifyRuleResponse, err error) {
    return ModifyRuleWithContext(context.Background(), c, request)
}

// ModifyRule
// 本接口为旧版本修改规则引擎接口，EdgeOne 于 2025 年 1 月 21 日已对规则引擎相关接口全面升级，新版本修改七层加速规则接口详情请参考 [ModifyL7AccRule](https://cloud.tencent.com/document/product/1552/115818)。
//
// <p style="color: red;">注意：自 2025 年 1 月 21 日起，旧版接口停止更新迭代，后续新增功能将仅在新版接口中提供，旧版接口支持的原有能力将不受影响。为避免在使用旧版接口时出现数据字段冲突，建议您尽早迁移到新版规则引擎接口。</p>
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_COMPRESSIONINVALIDALGORITHMS = "InvalidParameter.CompressionInvalidAlgorithms"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONDUPLICATEACTION = "InvalidParameter.ErrInvalidActionDuplicateAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMTOOMANYVALUES = "InvalidParameter.ErrInvalidActionParamTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONHOSTTOOMANYWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionHostTooManyWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDELSEWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidElseWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESECRETKEY = "InvalidParameter.InvalidAuthenticationTypeSecretKey"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEFORMAT = "InvalidParameter.InvalidAuthenticationTypeTimeFormat"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEPARAM = "InvalidParameter.InvalidAuthenticationTypeTimeParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYCOOKIE = "InvalidParameter.InvalidCacheKeyCookie"
//  INVALIDPARAMETER_INVALIDCACHEKEYIGNORECASE = "InvalidParameter.InvalidCacheKeyIgnoreCase"
//  INVALIDPARAMETER_INVALIDCACHEKEYSCHEME = "InvalidParameter.InvalidCacheKeyScheme"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDCOSDOMAIN = "InvalidParameter.InvalidCosDomain"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_ORIGINPULLPROTOCOLISREQUIRED = "InvalidParameter.OriginPullProtocolIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_RESPONSEHEADERCACHECONTROLNOTALLOWDELETE = "InvalidParameter.ResponseHeaderCacheControlNotAllowDelete"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETER_TLSVERSIONNOTINSEQUENCE = "InvalidParameter.TlsVersionNotInSequence"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_TRIALPLANRESPONSEPAGE = "InvalidParameterValue.TrialPlanResponsePage"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifyRuleWithContext(ctx context.Context, c *Client, request *ModifyRuleRequest) (response *ModifyRuleResponse, err error) {
    if request == nil {
        request = NewModifyRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyRuleResponse()
    err = c.Send(request, response)
    return
}

func NewModifySecurityAPIResourceRequest() (request *ModifySecurityAPIResourceRequest) {
    request = &ModifySecurityAPIResourceRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifySecurityAPIResource")
    
    
    return
}

func NewModifySecurityAPIResourceResponse() (response *ModifySecurityAPIResourceResponse) {
    response = &ModifySecurityAPIResourceResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifySecurityAPIResource
// 该接口用于修改 API 资源。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_COMPRESSIONINVALIDALGORITHMS = "InvalidParameter.CompressionInvalidAlgorithms"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONDUPLICATEACTION = "InvalidParameter.ErrInvalidActionDuplicateAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMTOOMANYVALUES = "InvalidParameter.ErrInvalidActionParamTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONHOSTTOOMANYWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionHostTooManyWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDELSEWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidElseWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESECRETKEY = "InvalidParameter.InvalidAuthenticationTypeSecretKey"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEFORMAT = "InvalidParameter.InvalidAuthenticationTypeTimeFormat"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEPARAM = "InvalidParameter.InvalidAuthenticationTypeTimeParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYCOOKIE = "InvalidParameter.InvalidCacheKeyCookie"
//  INVALIDPARAMETER_INVALIDCACHEKEYIGNORECASE = "InvalidParameter.InvalidCacheKeyIgnoreCase"
//  INVALIDPARAMETER_INVALIDCACHEKEYSCHEME = "InvalidParameter.InvalidCacheKeyScheme"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDCOSDOMAIN = "InvalidParameter.InvalidCosDomain"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_ORIGINPULLPROTOCOLISREQUIRED = "InvalidParameter.OriginPullProtocolIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_RESPONSEHEADERCACHECONTROLNOTALLOWDELETE = "InvalidParameter.ResponseHeaderCacheControlNotAllowDelete"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETER_TLSVERSIONNOTINSEQUENCE = "InvalidParameter.TlsVersionNotInSequence"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_TRIALPLANRESPONSEPAGE = "InvalidParameterValue.TrialPlanResponsePage"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifySecurityAPIResource(c *Client, request *ModifySecurityAPIResourceRequest) (response *ModifySecurityAPIResourceResponse, err error) {
    return ModifySecurityAPIResourceWithContext(context.Background(), c, request)
}

// ModifySecurityAPIResource
// 该接口用于修改 API 资源。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_COMPRESSIONINVALIDALGORITHMS = "InvalidParameter.CompressionInvalidAlgorithms"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONDUPLICATEACTION = "InvalidParameter.ErrInvalidActionDuplicateAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMTOOMANYVALUES = "InvalidParameter.ErrInvalidActionParamTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONHOSTTOOMANYWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionHostTooManyWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDELSEWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidElseWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESECRETKEY = "InvalidParameter.InvalidAuthenticationTypeSecretKey"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEFORMAT = "InvalidParameter.InvalidAuthenticationTypeTimeFormat"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEPARAM = "InvalidParameter.InvalidAuthenticationTypeTimeParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYCOOKIE = "InvalidParameter.InvalidCacheKeyCookie"
//  INVALIDPARAMETER_INVALIDCACHEKEYIGNORECASE = "InvalidParameter.InvalidCacheKeyIgnoreCase"
//  INVALIDPARAMETER_INVALIDCACHEKEYSCHEME = "InvalidParameter.InvalidCacheKeyScheme"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDCOSDOMAIN = "InvalidParameter.InvalidCosDomain"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_ORIGINPULLPROTOCOLISREQUIRED = "InvalidParameter.OriginPullProtocolIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_RESPONSEHEADERCACHECONTROLNOTALLOWDELETE = "InvalidParameter.ResponseHeaderCacheControlNotAllowDelete"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETER_TLSVERSIONNOTINSEQUENCE = "InvalidParameter.TlsVersionNotInSequence"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_TRIALPLANRESPONSEPAGE = "InvalidParameterValue.TrialPlanResponsePage"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifySecurityAPIResourceWithContext(ctx context.Context, c *Client, request *ModifySecurityAPIResourceRequest) (response *ModifySecurityAPIResourceResponse, err error) {
    if request == nil {
        request = NewModifySecurityAPIResourceRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifySecurityAPIResource")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifySecurityAPIResource require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifySecurityAPIResourceResponse()
    err = c.Send(request, response)
    return
}

func NewModifySecurityAPIServiceRequest() (request *ModifySecurityAPIServiceRequest) {
    request = &ModifySecurityAPIServiceRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifySecurityAPIService")
    
    
    return
}

func NewModifySecurityAPIServiceResponse() (response *ModifySecurityAPIServiceResponse) {
    response = &ModifySecurityAPIServiceResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifySecurityAPIService
// 该接口用于修改 API 服务。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_COMPRESSIONINVALIDALGORITHMS = "InvalidParameter.CompressionInvalidAlgorithms"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONDUPLICATEACTION = "InvalidParameter.ErrInvalidActionDuplicateAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMTOOMANYVALUES = "InvalidParameter.ErrInvalidActionParamTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONHOSTTOOMANYWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionHostTooManyWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDELSEWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidElseWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESECRETKEY = "InvalidParameter.InvalidAuthenticationTypeSecretKey"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEFORMAT = "InvalidParameter.InvalidAuthenticationTypeTimeFormat"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEPARAM = "InvalidParameter.InvalidAuthenticationTypeTimeParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYCOOKIE = "InvalidParameter.InvalidCacheKeyCookie"
//  INVALIDPARAMETER_INVALIDCACHEKEYIGNORECASE = "InvalidParameter.InvalidCacheKeyIgnoreCase"
//  INVALIDPARAMETER_INVALIDCACHEKEYSCHEME = "InvalidParameter.InvalidCacheKeyScheme"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDCOSDOMAIN = "InvalidParameter.InvalidCosDomain"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_ORIGINPULLPROTOCOLISREQUIRED = "InvalidParameter.OriginPullProtocolIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_RESPONSEHEADERCACHECONTROLNOTALLOWDELETE = "InvalidParameter.ResponseHeaderCacheControlNotAllowDelete"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETER_TLSVERSIONNOTINSEQUENCE = "InvalidParameter.TlsVersionNotInSequence"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_TRIALPLANRESPONSEPAGE = "InvalidParameterValue.TrialPlanResponsePage"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifySecurityAPIService(c *Client, request *ModifySecurityAPIServiceRequest) (response *ModifySecurityAPIServiceResponse, err error) {
    return ModifySecurityAPIServiceWithContext(context.Background(), c, request)
}

// ModifySecurityAPIService
// 该接口用于修改 API 服务。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_COMPRESSIONINVALIDALGORITHMS = "InvalidParameter.CompressionInvalidAlgorithms"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONDUPLICATEACTION = "InvalidParameter.ErrInvalidActionDuplicateAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMTOOMANYVALUES = "InvalidParameter.ErrInvalidActionParamTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONHOSTTOOMANYWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionHostTooManyWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDELSEWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidElseWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESECRETKEY = "InvalidParameter.InvalidAuthenticationTypeSecretKey"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEFORMAT = "InvalidParameter.InvalidAuthenticationTypeTimeFormat"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEPARAM = "InvalidParameter.InvalidAuthenticationTypeTimeParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYCOOKIE = "InvalidParameter.InvalidCacheKeyCookie"
//  INVALIDPARAMETER_INVALIDCACHEKEYIGNORECASE = "InvalidParameter.InvalidCacheKeyIgnoreCase"
//  INVALIDPARAMETER_INVALIDCACHEKEYSCHEME = "InvalidParameter.InvalidCacheKeyScheme"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDCOSDOMAIN = "InvalidParameter.InvalidCosDomain"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_ORIGINPULLPROTOCOLISREQUIRED = "InvalidParameter.OriginPullProtocolIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_RESPONSEHEADERCACHECONTROLNOTALLOWDELETE = "InvalidParameter.ResponseHeaderCacheControlNotAllowDelete"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETER_TLSVERSIONNOTINSEQUENCE = "InvalidParameter.TlsVersionNotInSequence"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_TRIALPLANRESPONSEPAGE = "InvalidParameterValue.TrialPlanResponsePage"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifySecurityAPIServiceWithContext(ctx context.Context, c *Client, request *ModifySecurityAPIServiceRequest) (response *ModifySecurityAPIServiceResponse, err error) {
    if request == nil {
        request = NewModifySecurityAPIServiceRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifySecurityAPIService")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifySecurityAPIService require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifySecurityAPIServiceResponse()
    err = c.Send(request, response)
    return
}

func NewModifySecurityClientAttesterRequest() (request *ModifySecurityClientAttesterRequest) {
    request = &ModifySecurityClientAttesterRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifySecurityClientAttester")
    
    
    return
}

func NewModifySecurityClientAttesterResponse() (response *ModifySecurityClientAttesterResponse) {
    response = &ModifySecurityClientAttesterResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifySecurityClientAttester
// 修改客户端认证选项。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_COMPRESSIONINVALIDALGORITHMS = "InvalidParameter.CompressionInvalidAlgorithms"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONDUPLICATEACTION = "InvalidParameter.ErrInvalidActionDuplicateAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMTOOMANYVALUES = "InvalidParameter.ErrInvalidActionParamTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONHOSTTOOMANYWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionHostTooManyWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDELSEWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidElseWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESECRETKEY = "InvalidParameter.InvalidAuthenticationTypeSecretKey"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEFORMAT = "InvalidParameter.InvalidAuthenticationTypeTimeFormat"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEPARAM = "InvalidParameter.InvalidAuthenticationTypeTimeParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYCOOKIE = "InvalidParameter.InvalidCacheKeyCookie"
//  INVALIDPARAMETER_INVALIDCACHEKEYIGNORECASE = "InvalidParameter.InvalidCacheKeyIgnoreCase"
//  INVALIDPARAMETER_INVALIDCACHEKEYSCHEME = "InvalidParameter.InvalidCacheKeyScheme"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDCOSDOMAIN = "InvalidParameter.InvalidCosDomain"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_ORIGINPULLPROTOCOLISREQUIRED = "InvalidParameter.OriginPullProtocolIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_RESPONSEHEADERCACHECONTROLNOTALLOWDELETE = "InvalidParameter.ResponseHeaderCacheControlNotAllowDelete"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETER_TLSVERSIONNOTINSEQUENCE = "InvalidParameter.TlsVersionNotInSequence"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_TRIALPLANRESPONSEPAGE = "InvalidParameterValue.TrialPlanResponsePage"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifySecurityClientAttester(c *Client, request *ModifySecurityClientAttesterRequest) (response *ModifySecurityClientAttesterResponse, err error) {
    return ModifySecurityClientAttesterWithContext(context.Background(), c, request)
}

// ModifySecurityClientAttester
// 修改客户端认证选项。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_COMPRESSIONINVALIDALGORITHMS = "InvalidParameter.CompressionInvalidAlgorithms"
//  INVALIDPARAMETER_ERRACTIONUNSUPPORTTARGET = "InvalidParameter.ErrActionUnsupportTarget"
//  INVALIDPARAMETER_ERRINVALIDACTION = "InvalidParameter.ErrInvalidAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONDUPLICATEACTION = "InvalidParameter.ErrInvalidActionDuplicateAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAM = "InvalidParameter.ErrInvalidActionParam"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMACTION = "InvalidParameter.ErrInvalidActionParamAction"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMDUPLICATENAME = "InvalidParameter.ErrInvalidActionParamDuplicateName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMNAME = "InvalidParameter.ErrInvalidActionParamName"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMTOOMANYVALUES = "InvalidParameter.ErrInvalidActionParamTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDACTIONPARAMVALUE = "InvalidParameter.ErrInvalidActionParamValue"
//  INVALIDPARAMETER_ERRINVALIDACTIONTYPE = "InvalidParameter.ErrInvalidActionType"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONHOSTTOOMANYWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidConditionHostTooManyWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONIGNORECASE = "InvalidParameter.ErrInvalidConditionIgnoreCase"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMEBADNAME = "InvalidParameter.ErrInvalidConditionNameBadName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONNAMETARGETNOTSUPPORTNAME = "InvalidParameter.ErrInvalidConditionNameTargetNotSupportName"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADREGULAR = "InvalidParameter.ErrInvalidConditionValueBadRegular"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADURL = "InvalidParameter.ErrInvalidConditionValueBadUrl"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUE = "InvalidParameter.ErrInvalidConditionValueBadValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUEBADVALUECONTAINFILENAMEEXTENSION = "InvalidParameter.ErrInvalidConditionValueBadValueContainFileNameExtension"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOLONGVALUE = "InvalidParameter.ErrInvalidConditionValueTooLongValue"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYVALUES = "InvalidParameter.ErrInvalidConditionValueTooManyValues"
//  INVALIDPARAMETER_ERRINVALIDCONDITIONVALUETOOMANYWILDCARD = "InvalidParameter.ErrInvalidConditionValueTooManyWildcard"
//  INVALIDPARAMETER_ERRINVALIDELSEWHENMODIFYORIGINACTIONCONFIGURED = "InvalidParameter.ErrInvalidElseWhenModifyOriginActionConfigured"
//  INVALIDPARAMETER_ERRNILCONDITION = "InvalidParameter.ErrNilCondition"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_HOSTNOTFOUND = "InvalidParameter.HostNotFound"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESECRETKEY = "InvalidParameter.InvalidAuthenticationTypeSecretKey"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPESIGNPARAM = "InvalidParameter.InvalidAuthenticationTypeSignParam"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEFORMAT = "InvalidParameter.InvalidAuthenticationTypeTimeFormat"
//  INVALIDPARAMETER_INVALIDAUTHENTICATIONTYPETIMEPARAM = "InvalidParameter.InvalidAuthenticationTypeTimeParam"
//  INVALIDPARAMETER_INVALIDAWSREGION = "InvalidParameter.InvalidAwsRegion"
//  INVALIDPARAMETER_INVALIDBACKUPSERVERNAME = "InvalidParameter.InvalidBackupServerName"
//  INVALIDPARAMETER_INVALIDCACHEKEY = "InvalidParameter.InvalidCacheKey"
//  INVALIDPARAMETER_INVALIDCACHEKEYCOOKIE = "InvalidParameter.InvalidCacheKeyCookie"
//  INVALIDPARAMETER_INVALIDCACHEKEYIGNORECASE = "InvalidParameter.InvalidCacheKeyIgnoreCase"
//  INVALIDPARAMETER_INVALIDCACHEKEYSCHEME = "InvalidParameter.InvalidCacheKeyScheme"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDCOSDOMAIN = "InvalidParameter.InvalidCosDomain"
//  INVALIDPARAMETER_INVALIDERRORPAGEREDIRECTURL = "InvalidParameter.InvalidErrorPageRedirectUrl"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAMEXFF = "InvalidParameter.InvalidRequestHeaderNameXff"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERVALUE = "InvalidParameter.InvalidRequestHeaderValue"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERNAME = "InvalidParameter.InvalidResponseHeaderName"
//  INVALIDPARAMETER_INVALIDRESPONSEHEADERVALUE = "InvalidParameter.InvalidResponseHeaderValue"
//  INVALIDPARAMETER_INVALIDRULEENGINEACTION = "InvalidParameter.InvalidRuleEngineAction"
//  INVALIDPARAMETER_INVALIDRULEENGINENOTFOUND = "InvalidParameter.InvalidRuleEngineNotFound"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGET = "InvalidParameter.InvalidRuleEngineTarget"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSEXTENSION = "InvalidParameter.InvalidRuleEngineTargetsExtension"
//  INVALIDPARAMETER_INVALIDRULEENGINETARGETSURL = "InvalidParameter.InvalidRuleEngineTargetsUrl"
//  INVALIDPARAMETER_INVALIDSERVERNAME = "InvalidParameter.InvalidServerName"
//  INVALIDPARAMETER_INVALIDUPSTREAMREQUESTQUERYSTRINGVALUE = "InvalidParameter.InvalidUpstreamRequestQueryStringValue"
//  INVALIDPARAMETER_INVALIDURLREDIRECTHOST = "InvalidParameter.InvalidUrlRedirectHost"
//  INVALIDPARAMETER_INVALIDURLREDIRECTURL = "InvalidParameter.InvalidUrlRedirectUrl"
//  INVALIDPARAMETER_KEYRULESINVALIDQUERYSTRINGVALUE = "InvalidParameter.KeyRulesInvalidQueryStringValue"
//  INVALIDPARAMETER_LOADBALANCEINSTANCEIDISREQUIRED = "InvalidParameter.LoadBalanceInstanceIdIsRequired"
//  INVALIDPARAMETER_NOTSUPPORTTHISPRESET = "InvalidParameter.NotSupportThisPreset"
//  INVALIDPARAMETER_ORIGINORIGINGROUPIDISREQUIRED = "InvalidParameter.OriginOriginGroupIdIsRequired"
//  INVALIDPARAMETER_ORIGINPULLPROTOCOLISREQUIRED = "InvalidParameter.OriginPullProtocolIsRequired"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_RESPONSEHEADERCACHECONTROLNOTALLOWDELETE = "InvalidParameter.ResponseHeaderCacheControlNotAllowDelete"
//  INVALIDPARAMETER_STATUSCODECACHEINVALIDSTATUSCODE = "InvalidParameter.StatusCodeCacheInvalidStatusCode"
//  INVALIDPARAMETER_TLSVERSIONNOTINSEQUENCE = "InvalidParameter.TlsVersionNotInSequence"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_TRIALPLANRESPONSEPAGE = "InvalidParameterValue.TrialPlanResponsePage"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  LIMITEXCEEDED = "LimitExceeded"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_INVALIDADVANCEDDEFENSESECURITYTYPE = "OperationDenied.InvalidAdvancedDefenseSecurityType"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifySecurityClientAttesterWithContext(ctx context.Context, c *Client, request *ModifySecurityClientAttesterRequest) (response *ModifySecurityClientAttesterResponse, err error) {
    if request == nil {
        request = NewModifySecurityClientAttesterRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifySecurityClientAttester")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifySecurityClientAttester require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifySecurityClientAttesterResponse()
    err = c.Send(request, response)
    return
}

func NewModifySecurityIPGroupRequest() (request *ModifySecurityIPGroupRequest) {
    request = &ModifySecurityIPGroupRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifySecurityIPGroup")
    
    
    return
}

func NewModifySecurityIPGroupResponse() (response *ModifySecurityIPGroupResponse) {
    response = &ModifySecurityIPGroupResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifySecurityIPGroup
// 修改安全 IP 组。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifySecurityIPGroup(c *Client, request *ModifySecurityIPGroupRequest) (response *ModifySecurityIPGroupResponse, err error) {
    return ModifySecurityIPGroupWithContext(context.Background(), c, request)
}

// ModifySecurityIPGroup
// 修改安全 IP 组。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifySecurityIPGroupWithContext(ctx context.Context, c *Client, request *ModifySecurityIPGroupRequest) (response *ModifySecurityIPGroupResponse, err error) {
    if request == nil {
        request = NewModifySecurityIPGroupRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifySecurityIPGroup")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifySecurityIPGroup require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifySecurityIPGroupResponse()
    err = c.Send(request, response)
    return
}

func NewModifySecurityJSInjectionRuleRequest() (request *ModifySecurityJSInjectionRuleRequest) {
    request = &ModifySecurityJSInjectionRuleRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifySecurityJSInjectionRule")
    
    
    return
}

func NewModifySecurityJSInjectionRuleResponse() (response *ModifySecurityJSInjectionRuleResponse) {
    response = &ModifySecurityJSInjectionRuleResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifySecurityJSInjectionRule
// 修改 JavaScript 注入规则。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifySecurityJSInjectionRule(c *Client, request *ModifySecurityJSInjectionRuleRequest) (response *ModifySecurityJSInjectionRuleResponse, err error) {
    return ModifySecurityJSInjectionRuleWithContext(context.Background(), c, request)
}

// ModifySecurityJSInjectionRule
// 修改 JavaScript 注入规则。
//
// 可能返回的错误码:
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
func ModifySecurityJSInjectionRuleWithContext(ctx context.Context, c *Client, request *ModifySecurityJSInjectionRuleRequest) (response *ModifySecurityJSInjectionRuleResponse, err error) {
    if request == nil {
        request = NewModifySecurityJSInjectionRuleRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifySecurityJSInjectionRule")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifySecurityJSInjectionRule require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifySecurityJSInjectionRuleResponse()
    err = c.Send(request, response)
    return
}

func NewModifySecurityPolicyRequest() (request *ModifySecurityPolicyRequest) {
    request = &ModifySecurityPolicyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifySecurityPolicy")
    
    
    return
}

func NewModifySecurityPolicyResponse() (response *ModifySecurityPolicyResponse) {
    response = &ModifySecurityPolicyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifySecurityPolicy
// 修改Web&Bot安全配置。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  LIMITEXCEEDED_SECURITY = "LimitExceeded.Security"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
//  UNSUPPORTEDOPERATION = "UnsupportedOperation"
func ModifySecurityPolicy(c *Client, request *ModifySecurityPolicyRequest) (response *ModifySecurityPolicyResponse, err error) {
    return ModifySecurityPolicyWithContext(context.Background(), c, request)
}

// ModifySecurityPolicy
// 修改Web&Bot安全配置。
//
// 可能返回的错误码:
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_PROXYSERVER = "InternalError.ProxyServer"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
//  LIMITEXCEEDED_SECURITY = "LimitExceeded.Security"
//  OPERATIONDENIED = "OperationDenied"
//  RESOURCEINUSE = "ResourceInUse"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
//  UNAUTHORIZEDOPERATION_UNKNOWN = "UnauthorizedOperation.Unknown"
//  UNSUPPORTEDOPERATION = "UnsupportedOperation"
func ModifySecurityPolicyWithContext(ctx context.Context, c *Client, request *ModifySecurityPolicyRequest) (response *ModifySecurityPolicyResponse, err error) {
    if request == nil {
        request = NewModifySecurityPolicyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifySecurityPolicy")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifySecurityPolicy require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifySecurityPolicyResponse()
    err = c.Send(request, response)
    return
}

func NewModifyWebSecurityTemplateRequest() (request *ModifyWebSecurityTemplateRequest) {
    request = &ModifyWebSecurityTemplateRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyWebSecurityTemplate")
    
    
    return
}

func NewModifyWebSecurityTemplateResponse() (response *ModifyWebSecurityTemplateResponse) {
    response = &ModifyWebSecurityTemplateResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyWebSecurityTemplate
// 修改安全策略配置模板
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
func ModifyWebSecurityTemplate(c *Client, request *ModifyWebSecurityTemplateRequest) (response *ModifyWebSecurityTemplateResponse, err error) {
    return ModifyWebSecurityTemplateWithContext(context.Background(), c, request)
}

// ModifyWebSecurityTemplate
// 修改安全策略配置模板
//
// 可能返回的错误码:
//  INVALIDPARAMETER_SECURITY = "InvalidParameter.Security"
func ModifyWebSecurityTemplateWithContext(ctx context.Context, c *Client, request *ModifyWebSecurityTemplateRequest) (response *ModifyWebSecurityTemplateResponse, err error) {
    if request == nil {
        request = NewModifyWebSecurityTemplateRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyWebSecurityTemplate")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyWebSecurityTemplate require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyWebSecurityTemplateResponse()
    err = c.Send(request, response)
    return
}

func NewModifyZoneRequest() (request *ModifyZoneRequest) {
    request = &ModifyZoneRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyZone")
    
    
    return
}

func NewModifyZoneResponse() (response *ModifyZoneResponse) {
    response = &ModifyZoneResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyZone
// 修改站点信息。
//
// 可能返回的错误码:
//  DRYRUNOPERATION = "DryRunOperation"
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_FAILEDTOCALLDNSPOD = "FailedOperation.FailedToCallDNSPod"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  INVALIDPARAMETER_ZONENAMEISREQUIRED = "InvalidParameter.ZoneNameIsRequired"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_ZONESAMEASNAME = "InvalidParameterValue.ZoneSameAsName"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DNSPODUNAUTHORIZEDROLEOPERATION = "OperationDenied.DNSPodUnauthorizedRoleOperation"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_DOMAININSHARECNAMEGROUP = "OperationDenied.DomainInShareCnameGroup"
//  OPERATIONDENIED_DOMAINNUMBERISNOTZERO = "OperationDenied.DomainNumberIsNotZero"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_MULTIPLECNAMEZONE = "OperationDenied.MultipleCnameZone"
//  OPERATIONDENIED_NSNOTALLOWTRAFFICSTRATEGY = "OperationDenied.NSNotAllowTrafficStrategy"
//  OPERATIONDENIED_NODOMAINACCESSZONEONLYALLOWMODIFIEDTOCNAME = "OperationDenied.NoDomainAccessZoneOnlyAllowModifiedToCNAME"
//  OPERATIONDENIED_NODOMAINACCESSZONEONLYSUPPORTMODIFYTYPE = "OperationDenied.NoDomainAccessZoneOnlySupportModifyType"
//  OPERATIONDENIED_PLANNOTSUPPORTMODIFYZONEAREA = "OperationDenied.PlanNotSupportModifyZoneArea"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  RESOURCEINUSE_CNAME = "ResourceInUse.Cname"
//  RESOURCEINUSE_DNS = "ResourceInUse.Dns"
//  RESOURCEINUSE_GENERICHOST = "ResourceInUse.GenericHost"
//  RESOURCEINUSE_NS = "ResourceInUse.NS"
//  RESOURCEINUSE_OTHERS = "ResourceInUse.Others"
//  RESOURCEINUSE_OTHERSALIASDOMAIN = "ResourceInUse.OthersAliasDomain"
//  RESOURCEINUSE_OTHERSCNAME = "ResourceInUse.OthersCname"
//  RESOURCEINUSE_OTHERSNS = "ResourceInUse.OthersNS"
//  RESOURCEINUSE_SELFANDOTHERSCNAME = "ResourceInUse.SelfAndOthersCname"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCENOTFOUND_DNSPODDOMAINNOTINACCOUNT = "ResourceNotFound.DNSPodDomainNotInAccount"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func ModifyZone(c *Client, request *ModifyZoneRequest) (response *ModifyZoneResponse, err error) {
    return ModifyZoneWithContext(context.Background(), c, request)
}

// ModifyZone
// 修改站点信息。
//
// 可能返回的错误码:
//  DRYRUNOPERATION = "DryRunOperation"
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_FAILEDTOCALLDNSPOD = "FailedOperation.FailedToCallDNSPod"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INVALIDPARAMETER_INVALIDORIGINIP = "InvalidParameter.InvalidOriginIp"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_LENGTHEXCEEDSLIMIT = "InvalidParameter.LengthExceedsLimit"
//  INVALIDPARAMETER_ZONENAMEISREQUIRED = "InvalidParameter.ZoneNameIsRequired"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  INVALIDPARAMETERVALUE_ZONESAMEASNAME = "InvalidParameterValue.ZoneSameAsName"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_DNSPODUNAUTHORIZEDROLEOPERATION = "OperationDenied.DNSPodUnauthorizedRoleOperation"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_DOMAININSHARECNAMEGROUP = "OperationDenied.DomainInShareCnameGroup"
//  OPERATIONDENIED_DOMAINNUMBERISNOTZERO = "OperationDenied.DomainNumberIsNotZero"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_MULTIPLECNAMEZONE = "OperationDenied.MultipleCnameZone"
//  OPERATIONDENIED_NSNOTALLOWTRAFFICSTRATEGY = "OperationDenied.NSNotAllowTrafficStrategy"
//  OPERATIONDENIED_NODOMAINACCESSZONEONLYALLOWMODIFIEDTOCNAME = "OperationDenied.NoDomainAccessZoneOnlyAllowModifiedToCNAME"
//  OPERATIONDENIED_NODOMAINACCESSZONEONLYSUPPORTMODIFYTYPE = "OperationDenied.NoDomainAccessZoneOnlySupportModifyType"
//  OPERATIONDENIED_PLANNOTSUPPORTMODIFYZONEAREA = "OperationDenied.PlanNotSupportModifyZoneArea"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  RESOURCEINUSE_CNAME = "ResourceInUse.Cname"
//  RESOURCEINUSE_DNS = "ResourceInUse.Dns"
//  RESOURCEINUSE_GENERICHOST = "ResourceInUse.GenericHost"
//  RESOURCEINUSE_NS = "ResourceInUse.NS"
//  RESOURCEINUSE_OTHERS = "ResourceInUse.Others"
//  RESOURCEINUSE_OTHERSALIASDOMAIN = "ResourceInUse.OthersAliasDomain"
//  RESOURCEINUSE_OTHERSCNAME = "ResourceInUse.OthersCname"
//  RESOURCEINUSE_OTHERSNS = "ResourceInUse.OthersNS"
//  RESOURCEINUSE_SELFANDOTHERSCNAME = "ResourceInUse.SelfAndOthersCname"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCENOTFOUND_DNSPODDOMAINNOTINACCOUNT = "ResourceNotFound.DNSPodDomainNotInAccount"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func ModifyZoneWithContext(ctx context.Context, c *Client, request *ModifyZoneRequest) (response *ModifyZoneResponse, err error) {
    if request == nil {
        request = NewModifyZoneRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyZone")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyZone require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyZoneResponse()
    err = c.Send(request, response)
    return
}

func NewModifyZoneSettingRequest() (request *ModifyZoneSettingRequest) {
    request = &ModifyZoneSettingRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyZoneSetting")
    
    
    return
}

func NewModifyZoneSettingResponse() (response *ModifyZoneSettingResponse) {
    response = &ModifyZoneSettingResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyZoneSetting
// 本接口为旧版，EdgeOne 已对规则引擎相关接口全面升级，详情请参考 [ModifyL7AccSetting](https://cloud.tencent.com/document/product/1552/115817)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CERTIFICATENOTFOUND = "FailedOperation.CertificateNotFound"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INTERNALERROR_UNKNOWERROR = "InternalError.UnknowError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGREQUIRESFULLURLCACHEOFF = "InvalidParameter.CacheKeyQueryStringRequiresFullUrlCacheOff"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_CLIENTIPCOUNTRYCONFLICTSWITHIPV6 = "InvalidParameter.ClientIpCountryConflictsWithIpv6"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_INVALIDAWSPRIVATEACCESS = "InvalidParameter.InvalidAwsPrivateAccess"
//  INVALIDPARAMETER_INVALIDCACHECONFIGFOLLOWORIGIN = "InvalidParameter.InvalidCacheConfigFollowOrigin"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHEONLYONSWITCH = "InvalidParameter.InvalidCacheOnlyOnSwitch"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPCOUNTRYHEADERNAME = "InvalidParameter.InvalidClientIpCountryHeaderName"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDFORCEREDIRECTTYPE = "InvalidParameter.InvalidForceRedirectType"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCERTINFO = "InvalidParameter.InvalidHttpsCertInfo"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDHTTPSTLSVERSION = "InvalidParameter.InvalidHttpsTlsVersion"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDORIGINTYPE = "InvalidParameter.InvalidOriginType"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTMAXSIZEBILLING = "InvalidParameter.InvalidPostMaxSizeBilling"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDRESOURCEIDBILLING = "InvalidParameter.InvalidResourceIdBilling"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGCLIENTIP = "InvalidParameter.InvalidStandardDebugClientIp"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEMPTYLIST = "InvalidParameter.InvalidStandardDebugEmptyList"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_MULTIPLYLAYERNOTSUPPORTSMARTROUTING = "InvalidParameter.MultiplyLayerNotSupportSmartRouting"
//  INVALIDPARAMETER_OCDIRECTORIGINREQUIRESSMARTROUTING = "InvalidParameter.OCDirectOriginRequiresSmartRouting"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_SETTINGINVALIDPARAM = "InvalidParameter.SettingInvalidParam"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDDISABLE = "OperationDenied.AccelerateMainlandDisable"
//  OPERATIONDENIED_ACCELERATEMAINLANDIPV6CONFLICT = "OperationDenied.AccelerateMainlandIpv6Conflict"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_SHAREDCNAMEUNSUPPORTEDACCELERATEMAINLAND = "OperationDenied.SharedCNAMEUnsupportedAccelerateMainland"
//  OPERATIONDENIED_SHAREDCNAMEUNSUPPORTEDIPV6 = "OperationDenied.SharedCNAMEUnsupportedIPv6"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCENOTFOUND_POSTMAXSIZEQUOTANOTFOUND = "ResourceNotFound.PostMaxSizeQuotaNotFound"
//  RESOURCEUNAVAILABLE_CERTNOTFOUND = "ResourceUnavailable.CertNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func ModifyZoneSetting(c *Client, request *ModifyZoneSettingRequest) (response *ModifyZoneSettingResponse, err error) {
    return ModifyZoneSettingWithContext(context.Background(), c, request)
}

// ModifyZoneSetting
// 本接口为旧版，EdgeOne 已对规则引擎相关接口全面升级，详情请参考 [ModifyL7AccSetting](https://cloud.tencent.com/document/product/1552/115817)。
//
// 可能返回的错误码:
//  FAILEDOPERATION = "FailedOperation"
//  FAILEDOPERATION_CERTIFICATENOTFOUND = "FailedOperation.CertificateNotFound"
//  INTERNALERROR_CONFIGLOCKED = "InternalError.ConfigLocked"
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  INTERNALERROR_SYSTEMERROR = "InternalError.SystemError"
//  INTERNALERROR_UNKNOWERROR = "InternalError.UnknowError"
//  INVALIDPARAMETER_ACTIONINPROGRESS = "InvalidParameter.ActionInProgress"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGREQUIRESFULLURLCACHEOFF = "InvalidParameter.CacheKeyQueryStringRequiresFullUrlCacheOff"
//  INVALIDPARAMETER_CACHEKEYQUERYSTRINGTOOMANYVALUE = "InvalidParameter.CacheKeyQueryStringTooManyValue"
//  INVALIDPARAMETER_CERTSYSTEMERROR = "InvalidParameter.CertSystemError"
//  INVALIDPARAMETER_CLIENTIPCOUNTRYCONFLICTSWITHIPV6 = "InvalidParameter.ClientIpCountryConflictsWithIpv6"
//  INVALIDPARAMETER_GRPCREQUIREHTTP2 = "InvalidParameter.GrpcRequireHttp2"
//  INVALIDPARAMETER_INVALIDAWSPRIVATEACCESS = "InvalidParameter.InvalidAwsPrivateAccess"
//  INVALIDPARAMETER_INVALIDCACHECONFIGFOLLOWORIGIN = "InvalidParameter.InvalidCacheConfigFollowOrigin"
//  INVALIDPARAMETER_INVALIDCACHEKEYQUERYSTRINGVALUE = "InvalidParameter.InvalidCacheKeyQueryStringValue"
//  INVALIDPARAMETER_INVALIDCACHEONLYONSWITCH = "InvalidParameter.InvalidCacheOnlyOnSwitch"
//  INVALIDPARAMETER_INVALIDCACHETIME = "InvalidParameter.InvalidCacheTime"
//  INVALIDPARAMETER_INVALIDCLIENTIPCOUNTRYHEADERNAME = "InvalidParameter.InvalidClientIpCountryHeaderName"
//  INVALIDPARAMETER_INVALIDCLIENTIPHEADERNAME = "InvalidParameter.InvalidClientIpHeaderName"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINE = "InvalidParameter.InvalidDynamicRoutine"
//  INVALIDPARAMETER_INVALIDDYNAMICROUTINEBILLING = "InvalidParameter.InvalidDynamicRoutineBilling"
//  INVALIDPARAMETER_INVALIDFORCEREDIRECTTYPE = "InvalidParameter.InvalidForceRedirectType"
//  INVALIDPARAMETER_INVALIDHTTPS = "InvalidParameter.InvalidHttps"
//  INVALIDPARAMETER_INVALIDHTTPSCERTINFO = "InvalidParameter.InvalidHttpsCertInfo"
//  INVALIDPARAMETER_INVALIDHTTPSCIPHERSUITEANDTLSVERSION = "InvalidParameter.InvalidHttpsCipherSuiteAndTlsVersion"
//  INVALIDPARAMETER_INVALIDHTTPSHSTSMAXAGE = "InvalidParameter.InvalidHttpsHstsMaxAge"
//  INVALIDPARAMETER_INVALIDHTTPSTLSVERSION = "InvalidParameter.InvalidHttpsTlsVersion"
//  INVALIDPARAMETER_INVALIDIPV6SWITCH = "InvalidParameter.InvalidIpv6Switch"
//  INVALIDPARAMETER_INVALIDMAXAGETIME = "InvalidParameter.InvalidMaxAgeTime"
//  INVALIDPARAMETER_INVALIDORIGIN = "InvalidParameter.InvalidOrigin"
//  INVALIDPARAMETER_INVALIDORIGINTYPE = "InvalidParameter.InvalidOriginType"
//  INVALIDPARAMETER_INVALIDPARAMETER = "InvalidParameter.InvalidParameter"
//  INVALIDPARAMETER_INVALIDPOSTMAXSIZEBILLING = "InvalidParameter.InvalidPostMaxSizeBilling"
//  INVALIDPARAMETER_INVALIDPOSTSIZEVALUE = "InvalidParameter.InvalidPostSizeValue"
//  INVALIDPARAMETER_INVALIDRANGEORIGINPULL = "InvalidParameter.InvalidRangeOriginPull"
//  INVALIDPARAMETER_INVALIDREQUESTHEADERNAME = "InvalidParameter.InvalidRequestHeaderName"
//  INVALIDPARAMETER_INVALIDRESOURCEIDBILLING = "InvalidParameter.InvalidResourceIdBilling"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGCLIENTIP = "InvalidParameter.InvalidStandardDebugClientIp"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEMPTYLIST = "InvalidParameter.InvalidStandardDebugEmptyList"
//  INVALIDPARAMETER_INVALIDSTANDARDDEBUGEXPIRETIMELIMIT = "InvalidParameter.InvalidStandardDebugExpireTimeLimit"
//  INVALIDPARAMETER_INVALIDWEBSOCKETTIMEOUT = "InvalidParameter.InvalidWebSocketTimeout"
//  INVALIDPARAMETER_MULTIPLYLAYERNOTSUPPORTSMARTROUTING = "InvalidParameter.MultiplyLayerNotSupportSmartRouting"
//  INVALIDPARAMETER_OCDIRECTORIGINREQUIRESSMARTROUTING = "InvalidParameter.OCDirectOriginRequiresSmartRouting"
//  INVALIDPARAMETER_POSTMAXSIZELIMITEXCEEDED = "InvalidParameter.PostMaxSizeLimitExceeded"
//  INVALIDPARAMETER_SETTINGINVALIDPARAM = "InvalidParameter.SettingInvalidParam"
//  INVALIDPARAMETER_ZONEISGRAYPUBLISHING = "InvalidParameter.ZoneIsGrayPublishing"
//  INVALIDPARAMETER_ZONENOTFOUND = "InvalidParameter.ZoneNotFound"
//  INVALIDPARAMETERVALUE_FORMATMISMATCH = "InvalidParameterValue.FormatMismatch"
//  INVALIDPARAMETERVALUE_GENERALMISMATCH = "InvalidParameterValue.GeneralMismatch"
//  INVALIDPARAMETERVALUE_INCLUDEINVALIDVALUE = "InvalidParameterValue.IncludeInvalidValue"
//  INVALIDPARAMETERVALUE_MISSINGNECESSARYPARAM = "InvalidParameterValue.MissingNecessaryParam"
//  INVALIDPARAMETERVALUE_NOTINENUMERATION = "InvalidParameterValue.NotInEnumeration"
//  INVALIDPARAMETERVALUE_NOTWITHINRANGE = "InvalidParameterValue.NotWithinRange"
//  INVALIDPARAMETERVALUE_REGEXMISMATCH = "InvalidParameterValue.RegExMismatch"
//  INVALIDPARAMETERVALUE_UNRECOGNIZABLEVALUE = "InvalidParameterValue.UnrecognizableValue"
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_ACCELERATEMAINLANDDISABLE = "OperationDenied.AccelerateMainlandDisable"
//  OPERATIONDENIED_ACCELERATEMAINLANDIPV6CONFLICT = "OperationDenied.AccelerateMainlandIpv6Conflict"
//  OPERATIONDENIED_ACCELERATEMAINLANDMULTIPLYLAYERCONFLICT = "OperationDenied.AccelerateMainlandMultiplyLayerConflict"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_ERRZONEISALREADYPAUSED = "OperationDenied.ErrZoneIsAlreadyPaused"
//  OPERATIONDENIED_SHAREDCNAMEUNSUPPORTEDACCELERATEMAINLAND = "OperationDenied.SharedCNAMEUnsupportedAccelerateMainland"
//  OPERATIONDENIED_SHAREDCNAMEUNSUPPORTEDIPV6 = "OperationDenied.SharedCNAMEUnsupportedIPv6"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCENOTFOUND_POSTMAXSIZEQUOTANOTFOUND = "ResourceNotFound.PostMaxSizeQuotaNotFound"
//  RESOURCEUNAVAILABLE_CERTNOTFOUND = "ResourceUnavailable.CertNotFound"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
//  UNAUTHORIZEDOPERATION_NOPERMISSION = "UnauthorizedOperation.NoPermission"
func ModifyZoneSettingWithContext(ctx context.Context, c *Client, request *ModifyZoneSettingRequest) (response *ModifyZoneSettingResponse, err error) {
    if request == nil {
        request = NewModifyZoneSettingRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyZoneSetting")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyZoneSetting require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyZoneSettingResponse()
    err = c.Send(request, response)
    return
}

func NewModifyZoneStatusRequest() (request *ModifyZoneStatusRequest) {
    request = &ModifyZoneStatusRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "ModifyZoneStatus")
    
    
    return
}

func NewModifyZoneStatusResponse() (response *ModifyZoneStatusResponse) {
    response = &ModifyZoneStatusResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// ModifyZoneStatus
// 用于开启，关闭站点。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_L4PROXYINPROGRESSSTATUS = "OperationDenied.L4ProxyInProgressStatus"
//  OPERATIONDENIED_L4PROXYINSTOPPINGSTATUS = "OperationDenied.L4ProxyInStoppingStatus"
//  OPERATIONDENIED_L7HOSTINPROCESSSTATUS = "OperationDenied.L7HostInProcessStatus"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func ModifyZoneStatus(c *Client, request *ModifyZoneStatusRequest) (response *ModifyZoneStatusResponse, err error) {
    return ModifyZoneStatusWithContext(context.Background(), c, request)
}

// ModifyZoneStatus
// 用于开启，关闭站点。
//
// 可能返回的错误码:
//  OPERATIONDENIED = "OperationDenied"
//  OPERATIONDENIED_CONFIGLOCKED = "OperationDenied.ConfigLocked"
//  OPERATIONDENIED_DISABLEZONENOTCOMPLETED = "OperationDenied.DisableZoneNotCompleted"
//  OPERATIONDENIED_L4PROXYINPROGRESSSTATUS = "OperationDenied.L4ProxyInProgressStatus"
//  OPERATIONDENIED_L4PROXYINSTOPPINGSTATUS = "OperationDenied.L4ProxyInStoppingStatus"
//  OPERATIONDENIED_L7HOSTINPROCESSSTATUS = "OperationDenied.L7HostInProcessStatus"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  OPERATIONDENIED_VERSIONCONTROLISGRAYING = "OperationDenied.VersionControlIsGraying"
//  RESOURCEINUSE = "ResourceInUse"
//  RESOURCENOTFOUND = "ResourceNotFound"
//  RESOURCEUNAVAILABLE = "ResourceUnavailable"
//  UNAUTHORIZEDOPERATION_CAMUNAUTHORIZED = "UnauthorizedOperation.CamUnauthorized"
func ModifyZoneStatusWithContext(ctx context.Context, c *Client, request *ModifyZoneStatusRequest) (response *ModifyZoneStatusResponse, err error) {
    if request == nil {
        request = NewModifyZoneStatusRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "ModifyZoneStatus")
    
    if c.GetCredential() == nil {
        return nil, errors.New("ModifyZoneStatus require credential")
    }

    request.SetContext(ctx)
    
    response = NewModifyZoneStatusResponse()
    err = c.Send(request, response)
    return
}

func NewRefreshMultiPathGatewaySecretKeyRequest() (request *RefreshMultiPathGatewaySecretKeyRequest) {
    request = &RefreshMultiPathGatewaySecretKeyRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "RefreshMultiPathGatewaySecretKey")
    
    
    return
}

func NewRefreshMultiPathGatewaySecretKeyResponse() (response *RefreshMultiPathGatewaySecretKeyResponse) {
    response = &RefreshMultiPathGatewaySecretKeyResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// RefreshMultiPathGatewaySecretKey
// 通过本接口刷新多通道安全加速网关的密钥。客户基于接入密钥签名接入多通道安全加速网关。每个站点下只有一个密钥，可用于接入该站点下的所有网关，刷新密钥后，原始密钥会失效。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func RefreshMultiPathGatewaySecretKey(c *Client, request *RefreshMultiPathGatewaySecretKeyRequest) (response *RefreshMultiPathGatewaySecretKeyResponse, err error) {
    return RefreshMultiPathGatewaySecretKeyWithContext(context.Background(), c, request)
}

// RefreshMultiPathGatewaySecretKey
// 通过本接口刷新多通道安全加速网关的密钥。客户基于接入密钥签名接入多通道安全加速网关。每个站点下只有一个密钥，可用于接入该站点下的所有网关，刷新密钥后，原始密钥会失效。
//
// 可能返回的错误码:
//  INTERNALERROR = "InternalError"
//  INVALIDPARAMETER = "InvalidParameter"
//  INVALIDPARAMETERVALUE = "InvalidParameterValue"
//  MISSINGPARAMETER = "MissingParameter"
//  OPERATIONDENIED = "OperationDenied"
func RefreshMultiPathGatewaySecretKeyWithContext(ctx context.Context, c *Client, request *RefreshMultiPathGatewaySecretKeyRequest) (response *RefreshMultiPathGatewaySecretKeyResponse, err error) {
    if request == nil {
        request = NewRefreshMultiPathGatewaySecretKeyRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "RefreshMultiPathGatewaySecretKey")
    
    if c.GetCredential() == nil {
        return nil, errors.New("RefreshMultiPathGatewaySecretKey require credential")
    }

    request.SetContext(ctx)
    
    response = NewRefreshMultiPathGatewaySecretKeyResponse()
    err = c.Send(request, response)
    return
}

func NewRenewPlanRequest() (request *RenewPlanRequest) {
    request = &RenewPlanRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "RenewPlan")
    
    
    return
}

func NewRenewPlanResponse() (response *RenewPlanResponse) {
    response = &RenewPlanResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// RenewPlan
// 当您的套餐需要延长有效期，可以通过该接口进行续费。套餐续费仅支持个人版，基础版，标准版套餐。
//
// > 费用详情可参考 [套餐费用](https://cloud.tencent.com/document/product/1552/94158)
//
// 可能返回的错误码:
//  FAILEDOPERATION_INSUFFICIENTACCOUNTBALANCE = "FailedOperation.InsufficientAccountBalance"
//  INVALIDPARAMETER_INVALIDAUTOUSEVOUCHER = "InvalidParameter.InvalidAutoUseVoucher"
//  INVALIDPARAMETER_INVALIDPERIOD = "InvalidParameter.InvalidPeriod"
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  OPERATIONDENIED_ENTERPRISEPLANRENEWUNSUPPORTED = "OperationDenied.EnterprisePlanRenewUnsupported"
//  OPERATIONDENIED_RESOURCEHASBEENLOCKED = "OperationDenied.ResourceHasBeenLocked"
func RenewPlan(c *Client, request *RenewPlanRequest) (response *RenewPlanResponse, err error) {
    return RenewPlanWithContext(context.Background(), c, request)
}

// RenewPlan
// 当您的套餐需要延长有效期，可以通过该接口进行续费。套餐续费仅支持个人版，基础版，标准版套餐。
//
// > 费用详情可参考 [套餐费用](https://cloud.tencent.com/document/product/1552/94158)
//
// 可能返回的错误码:
//  FAILEDOPERATION_INSUFFICIENTACCOUNTBALANCE = "FailedOperation.InsufficientAccountBalance"
//  INVALIDPARAMETER_INVALIDAUTOUSEVOUCHER = "InvalidParameter.InvalidAutoUseVoucher"
//  INVALIDPARAMETER_INVALIDPERIOD = "InvalidParameter.InvalidPeriod"
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  OPERATIONDENIED_ENTERPRISEPLANRENEWUNSUPPORTED = "OperationDenied.EnterprisePlanRenewUnsupported"
//  OPERATIONDENIED_RESOURCEHASBEENLOCKED = "OperationDenied.ResourceHasBeenLocked"
func RenewPlanWithContext(ctx context.Context, c *Client, request *RenewPlanRequest) (response *RenewPlanResponse, err error) {
    if request == nil {
        request = NewRenewPlanRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "RenewPlan")
    
    if c.GetCredential() == nil {
        return nil, errors.New("RenewPlan require credential")
    }

    request.SetContext(ctx)
    
    response = NewRenewPlanResponse()
    err = c.Send(request, response)
    return
}

func NewUpgradePlanRequest() (request *UpgradePlanRequest) {
    request = &UpgradePlanRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "UpgradePlan")
    
    
    return
}

func NewUpgradePlanResponse() (response *UpgradePlanResponse) {
    response = &UpgradePlanResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// UpgradePlan
// 当您需要使用高等级套餐才拥有的功能，可以通过本接口升级套餐，仅支持个人版，基础版套餐进行升级。
//
// > 不同类型 Edgeone 计费套餐区别参考 [Edgeone计费套餐选型对比](https://cloud.tencent.com/document/product/1552/94165)
//
// 计费套餐升级规则以及资费详情参考 [Edgeone计费套餐升配说明](https://cloud.tencent.com/document/product/1552/95291)
//
// 如果需要将套餐升级至企业版，请 [联系我们](https://cloud.tencent.com/online-service)
//
// 可能返回的错误码:
//  FAILEDOPERATION_INSUFFICIENTACCOUNTBALANCE = "FailedOperation.InsufficientAccountBalance"
//  INVALIDPARAMETER_INVALIDAUTOUSEVOUCHER = "InvalidParameter.InvalidAutoUseVoucher"
//  INVALIDPARAMETER_INVALIDPLANTYPE = "InvalidParameter.InvalidPlanType"
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  OPERATIONDENIED_ENTERPRISEPLANUPGRADEUNSUPPORTED = "OperationDenied.EnterprisePlanUpgradeUnsupported"
//  OPERATIONDENIED_PLANDOWNGRADENOTALLOWED = "OperationDenied.PlanDowngradeNotAllowed"
//  OPERATIONDENIED_PLANHASBEENEXPIRED = "OperationDenied.PlanHasBeenExpired"
//  OPERATIONDENIED_PLANHASBEENISOLATED = "OperationDenied.PlanHasBeenIsolated"
//  OPERATIONDENIED_RESOURCEHASBEENLOCKED = "OperationDenied.ResourceHasBeenLocked"
func UpgradePlan(c *Client, request *UpgradePlanRequest) (response *UpgradePlanResponse, err error) {
    return UpgradePlanWithContext(context.Background(), c, request)
}

// UpgradePlan
// 当您需要使用高等级套餐才拥有的功能，可以通过本接口升级套餐，仅支持个人版，基础版套餐进行升级。
//
// > 不同类型 Edgeone 计费套餐区别参考 [Edgeone计费套餐选型对比](https://cloud.tencent.com/document/product/1552/94165)
//
// 计费套餐升级规则以及资费详情参考 [Edgeone计费套餐升配说明](https://cloud.tencent.com/document/product/1552/95291)
//
// 如果需要将套餐升级至企业版，请 [联系我们](https://cloud.tencent.com/online-service)
//
// 可能返回的错误码:
//  FAILEDOPERATION_INSUFFICIENTACCOUNTBALANCE = "FailedOperation.InsufficientAccountBalance"
//  INVALIDPARAMETER_INVALIDAUTOUSEVOUCHER = "InvalidParameter.InvalidAutoUseVoucher"
//  INVALIDPARAMETER_INVALIDPLANTYPE = "InvalidParameter.InvalidPlanType"
//  INVALIDPARAMETER_PLANNOTFOUND = "InvalidParameter.PlanNotFound"
//  OPERATIONDENIED_ENTERPRISEPLANUPGRADEUNSUPPORTED = "OperationDenied.EnterprisePlanUpgradeUnsupported"
//  OPERATIONDENIED_PLANDOWNGRADENOTALLOWED = "OperationDenied.PlanDowngradeNotAllowed"
//  OPERATIONDENIED_PLANHASBEENEXPIRED = "OperationDenied.PlanHasBeenExpired"
//  OPERATIONDENIED_PLANHASBEENISOLATED = "OperationDenied.PlanHasBeenIsolated"
//  OPERATIONDENIED_RESOURCEHASBEENLOCKED = "OperationDenied.ResourceHasBeenLocked"
func UpgradePlanWithContext(ctx context.Context, c *Client, request *UpgradePlanRequest) (response *UpgradePlanResponse, err error) {
    if request == nil {
        request = NewUpgradePlanRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "UpgradePlan")
    
    if c.GetCredential() == nil {
        return nil, errors.New("UpgradePlan require credential")
    }

    request.SetContext(ctx)
    
    response = NewUpgradePlanResponse()
    err = c.Send(request, response)
    return
}

func NewVerifyOwnershipRequest() (request *VerifyOwnershipRequest) {
    request = &VerifyOwnershipRequest{
        BaseRequest: &tchttp.BaseRequest{},
    }
    
    request.Init().WithApiInfo("teo", APIVersion, "VerifyOwnership")
    
    
    return
}

func NewVerifyOwnershipResponse() (response *VerifyOwnershipResponse) {
    response = &VerifyOwnershipResponse{
        BaseResponse: &tchttp.BaseResponse{},
    } 
    return

}

// VerifyOwnership
// 在 CNAME 接入模式下，您需要对站点或者域名的归属权进行验证，可以通过本接口触发验证。若站点通过归属权验证后，后续添加域名无需再验证。详情参考 [站点/域名归属权验证](https://cloud.tencent.com/document/product/1552/70789)。
//
// 
//
// 在 NS 接入模式下，您也可以通过本接口来查询 NS 服务器是否切换成功，详情参考 [修改 DNS 服务器](https://cloud.tencent.com/document/product/1552/90452)。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  RESOURCENOTFOUND = "ResourceNotFound"
func VerifyOwnership(c *Client, request *VerifyOwnershipRequest) (response *VerifyOwnershipResponse, err error) {
    return VerifyOwnershipWithContext(context.Background(), c, request)
}

// VerifyOwnership
// 在 CNAME 接入模式下，您需要对站点或者域名的归属权进行验证，可以通过本接口触发验证。若站点通过归属权验证后，后续添加域名无需再验证。详情参考 [站点/域名归属权验证](https://cloud.tencent.com/document/product/1552/70789)。
//
// 
//
// 在 NS 接入模式下，您也可以通过本接口来查询 NS 服务器是否切换成功，详情参考 [修改 DNS 服务器](https://cloud.tencent.com/document/product/1552/90452)。
//
// 可能返回的错误码:
//  INTERNALERROR_ROUTEERROR = "InternalError.RouteError"
//  OPERATIONDENIED_RESOURCELOCKEDTEMPORARY = "OperationDenied.ResourceLockedTemporary"
//  RESOURCENOTFOUND = "ResourceNotFound"
func VerifyOwnershipWithContext(ctx context.Context, c *Client, request *VerifyOwnershipRequest) (response *VerifyOwnershipResponse, err error) {
    if request == nil {
        request = NewVerifyOwnershipRequest()
    }
    c.InitBaseRequest(&request.BaseRequest, "teo", APIVersion, "VerifyOwnership")
    
    if c.GetCredential() == nil {
        return nil, errors.New("VerifyOwnership require credential")
    }

    request.SetContext(ctx)
    
    response = NewVerifyOwnershipResponse()
    err = c.Send(request, response)
    return
}
