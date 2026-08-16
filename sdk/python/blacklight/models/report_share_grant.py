from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="ReportShareGrant")


@_attrs_define
class ReportShareGrant:
    id: UUID
    share_id: UUID
    created_at: datetime.datetime
    user_id: None | Unset | UUID = UNSET
    """ The user who claimed this grant, or null if unclaimed. """
    claimed_at: datetime.datetime | None | Unset = UNSET
    revoked_at: datetime.datetime | None | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        share_id = str(self.share_id)

        created_at = self.created_at.isoformat()

        user_id: None | str | Unset
        if isinstance(self.user_id, Unset):
            user_id = UNSET
        elif isinstance(self.user_id, UUID):
            user_id = str(self.user_id)
        else:
            user_id = self.user_id

        claimed_at: None | str | Unset
        if isinstance(self.claimed_at, Unset):
            claimed_at = UNSET
        elif isinstance(self.claimed_at, datetime.datetime):
            claimed_at = self.claimed_at.isoformat()
        else:
            claimed_at = self.claimed_at

        revoked_at: None | str | Unset
        if isinstance(self.revoked_at, Unset):
            revoked_at = UNSET
        elif isinstance(self.revoked_at, datetime.datetime):
            revoked_at = self.revoked_at.isoformat()
        else:
            revoked_at = self.revoked_at

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "id": id,
                "shareId": share_id,
                "createdAt": created_at,
            }
        )
        if user_id is not UNSET:
            field_dict["userId"] = user_id
        if claimed_at is not UNSET:
            field_dict["claimedAt"] = claimed_at
        if revoked_at is not UNSET:
            field_dict["revokedAt"] = revoked_at

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        share_id = UUID(d.pop("shareId"))

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        def _parse_user_id(data: object) -> None | Unset | UUID:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                user_id_type_0 = UUID(data)

                return user_id_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(None | Unset | UUID, data)

        user_id = _parse_user_id(d.pop("userId", UNSET))

        def _parse_claimed_at(data: object) -> datetime.datetime | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                claimed_at_type_0 = datetime.datetime.fromisoformat(data)

                return claimed_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None | Unset, data)

        claimed_at = _parse_claimed_at(d.pop("claimedAt", UNSET))

        def _parse_revoked_at(data: object) -> datetime.datetime | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                revoked_at_type_0 = datetime.datetime.fromisoformat(data)

                return revoked_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None | Unset, data)

        revoked_at = _parse_revoked_at(d.pop("revokedAt", UNSET))

        report_share_grant = cls(
            id=id,
            share_id=share_id,
            created_at=created_at,
            user_id=user_id,
            claimed_at=claimed_at,
            revoked_at=revoked_at,
        )

        return report_share_grant
