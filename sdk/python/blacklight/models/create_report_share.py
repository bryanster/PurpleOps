from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast
import datetime


T = TypeVar("T", bound="CreateReportShare")


@_attrs_define
class CreateReportShare:
    """Body of POST /report-versions/{versionId}/shares."""

    password: str | Unset = UNSET
    """ Optional password gate. When set, must satisfy the account password
    policy (internal/authn/password): at least 12 characters, at most
    128, and not a common password. Omit for a share with no gate.
     """
    expires_at: datetime.datetime | Unset = UNSET
    """ Optional expiry. After this time the share returns 404. """
    label: str | Unset = UNSET
    """ Optional human-readable label. """
    max_grants: int | Unset = UNSET
    """ Maximum number of claims. Omit for unlimited. """

    def to_dict(self) -> dict[str, Any]:
        password = self.password

        expires_at: str | Unset = UNSET
        if not isinstance(self.expires_at, Unset):
            expires_at = self.expires_at.isoformat()

        label = self.label

        max_grants = self.max_grants

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if password is not UNSET:
            field_dict["password"] = password
        if expires_at is not UNSET:
            field_dict["expiresAt"] = expires_at
        if label is not UNSET:
            field_dict["label"] = label
        if max_grants is not UNSET:
            field_dict["maxGrants"] = max_grants

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        password = d.pop("password", UNSET)

        _expires_at = d.pop("expiresAt", UNSET)
        expires_at: datetime.datetime | Unset
        if isinstance(_expires_at, Unset):
            expires_at = UNSET
        else:
            expires_at = datetime.datetime.fromisoformat(_expires_at)

        label = d.pop("label", UNSET)

        max_grants = d.pop("maxGrants", UNSET)

        create_report_share = cls(
            password=password,
            expires_at=expires_at,
            label=label,
            max_grants=max_grants,
        )

        return create_report_share
