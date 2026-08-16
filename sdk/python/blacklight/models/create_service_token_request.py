from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.token_scope import TokenScope
from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="CreateServiceTokenRequest")


@_attrs_define
class CreateServiceTokenRequest:
    """Body of `POST /auth/tokens`. The owner is the caller and is not a field."""

    name: str
    """ How you will recognise this token when you come to revoke it. """
    scopes: list[TokenScope]
    """ What the token may do. Repeats are accepted and stored once; a scope
    this server does not define is a `400` rather than a silently
    narrower token.
     """
    expires_at: datetime.datetime
    """ When the token should stop working. Required, must be in the future,
    and at most one year out — `400` otherwise, naming the field.
     """
    engagement_id: UUID | Unset = UNSET
    """ Bind the token to one engagement. Omit for a token that may reach
    every engagement you can. It cannot widen what you may reach.
     """

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        scopes = []
        for scopes_item_data in self.scopes:
            scopes_item = scopes_item_data.value
            scopes.append(scopes_item)

        expires_at = self.expires_at.isoformat()

        engagement_id: str | Unset = UNSET
        if not isinstance(self.engagement_id, Unset):
            engagement_id = str(self.engagement_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
                "scopes": scopes,
                "expiresAt": expires_at,
            }
        )
        if engagement_id is not UNSET:
            field_dict["engagementId"] = engagement_id

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        name = d.pop("name")

        scopes = []
        _scopes = d.pop("scopes")
        for scopes_item_data in _scopes:
            scopes_item = TokenScope(scopes_item_data)

            scopes.append(scopes_item)

        expires_at = datetime.datetime.fromisoformat(d.pop("expiresAt"))

        _engagement_id = d.pop("engagementId", UNSET)
        engagement_id: UUID | Unset
        if isinstance(_engagement_id, Unset):
            engagement_id = UNSET
        else:
            engagement_id = UUID(_engagement_id)

        create_service_token_request = cls(
            name=name,
            scopes=scopes,
            expires_at=expires_at,
            engagement_id=engagement_id,
        )

        return create_service_token_request
