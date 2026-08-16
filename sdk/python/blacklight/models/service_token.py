from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.service_token_status import ServiceTokenStatus
from ..models.token_scope import TokenScope
from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="ServiceToken")


@_attrs_define
class ServiceToken:
    """One service token, as anybody but its creator ever sees it: everything
    about the credential except the credential.

    """

    id: UUID
    """ The token's identifier, and what `DELETE /auth/tokens/{tokenId}` names. Not a credential. """
    name: str
    """ What its owner called it, so they can tell which one to revoke. """
    prefix: str
    """ The public identifier in the middle of the token — the `xxx` of
    `bl_xxx_…`. It is how a token found in a log or a CI variable is
    matched to a row here without anybody handling its secret.
     """
    scopes: list[TokenScope]
    """ What this token may do, subject to what its owner may do. """
    status: ServiceTokenStatus
    """ Whether a token works right now, derived from its timestamps so that a
    client does not have to compare dates to render a list. `revoked` wins
    over `expired` where both are true: it is the fact somebody acted on.
     """
    created_at: datetime.datetime
    expires_at: datetime.datetime
    """ When it stops working. Always set — a token with no expiry cannot be created. """
    engagement_id: UUID | Unset = UNSET
    """ The one engagement this token may touch. Absent on a token that may
    reach every engagement its owner can. It only ever subtracts.
     """
    last_used_at: datetime.datetime | Unset = UNSET
    """ When it was last used. Absent if it never has been. Accurate to
    within a minute and deliberately no more: recording every request
    would put a database write behind every request, and this column
    answers "is this still in use?", which does not need the seconds.
     """
    revoked_at: datetime.datetime | Unset = UNSET
    """ When somebody ended it early. Absent on a token nobody has revoked;
    expired and revoked are different facts and `status` says which.
     """
    revoked_by: UUID | Unset = UNSET
    """ Which account ended it. Absent on a token nobody has revoked, and
    equal to the owner on one its owner rotated themselves — so a value
    here that is *not* the owner is an administrator having stepped in
    (`DELETE /users/{userId}/tokens/{tokenId}`), which is the question
    an incident review asks of the credential in front of it.
     """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        name = self.name

        prefix = self.prefix

        scopes = []
        for scopes_item_data in self.scopes:
            scopes_item = scopes_item_data.value
            scopes.append(scopes_item)

        status = self.status.value

        created_at = self.created_at.isoformat()

        expires_at = self.expires_at.isoformat()

        engagement_id: str | Unset = UNSET
        if not isinstance(self.engagement_id, Unset):
            engagement_id = str(self.engagement_id)

        last_used_at: str | Unset = UNSET
        if not isinstance(self.last_used_at, Unset):
            last_used_at = self.last_used_at.isoformat()

        revoked_at: str | Unset = UNSET
        if not isinstance(self.revoked_at, Unset):
            revoked_at = self.revoked_at.isoformat()

        revoked_by: str | Unset = UNSET
        if not isinstance(self.revoked_by, Unset):
            revoked_by = str(self.revoked_by)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "name": name,
                "prefix": prefix,
                "scopes": scopes,
                "status": status,
                "createdAt": created_at,
                "expiresAt": expires_at,
            }
        )
        if engagement_id is not UNSET:
            field_dict["engagementId"] = engagement_id
        if last_used_at is not UNSET:
            field_dict["lastUsedAt"] = last_used_at
        if revoked_at is not UNSET:
            field_dict["revokedAt"] = revoked_at
        if revoked_by is not UNSET:
            field_dict["revokedBy"] = revoked_by

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        name = d.pop("name")

        prefix = d.pop("prefix")

        scopes = []
        _scopes = d.pop("scopes")
        for scopes_item_data in _scopes:
            scopes_item = TokenScope(scopes_item_data)

            scopes.append(scopes_item)

        status = ServiceTokenStatus(d.pop("status"))

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        expires_at = datetime.datetime.fromisoformat(d.pop("expiresAt"))

        _engagement_id = d.pop("engagementId", UNSET)
        engagement_id: UUID | Unset
        if isinstance(_engagement_id, Unset):
            engagement_id = UNSET
        else:
            engagement_id = UUID(_engagement_id)

        _last_used_at = d.pop("lastUsedAt", UNSET)
        last_used_at: datetime.datetime | Unset
        if isinstance(_last_used_at, Unset):
            last_used_at = UNSET
        else:
            last_used_at = datetime.datetime.fromisoformat(_last_used_at)

        _revoked_at = d.pop("revokedAt", UNSET)
        revoked_at: datetime.datetime | Unset
        if isinstance(_revoked_at, Unset):
            revoked_at = UNSET
        else:
            revoked_at = datetime.datetime.fromisoformat(_revoked_at)

        _revoked_by = d.pop("revokedBy", UNSET)
        revoked_by: UUID | Unset
        if isinstance(_revoked_by, Unset):
            revoked_by = UNSET
        else:
            revoked_by = UUID(_revoked_by)

        service_token = cls(
            id=id,
            name=name,
            prefix=prefix,
            scopes=scopes,
            status=status,
            created_at=created_at,
            expires_at=expires_at,
            engagement_id=engagement_id,
            last_used_at=last_used_at,
            revoked_at=revoked_at,
            revoked_by=revoked_by,
        )

        service_token.additional_properties = d
        return service_token

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
